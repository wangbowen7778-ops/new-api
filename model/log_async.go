package model

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Billing and audit logs back reconciliation, so this queue favours durability
// over throughput: a full queue writes inline instead of dropping the row, and
// the batch writer retries before giving up. Only a database that keeps
// rejecting writes can lose a row, and that is reported as a system error.
const (
	logQueueCapacity  = 20000
	logBatchSize      = 200
	logFlushInterval  = 200 * time.Millisecond
	logWriteAttempts  = 3
	logWriteRetryWait = 200 * time.Millisecond
)

var (
	logQueueOnce sync.Once
	logQueue     chan *Log
	logWorkerWG  sync.WaitGroup
	logStop      chan struct{}
	logDrained   chan struct{}
	logStopped   atomic.Bool

	logFlushOnce sync.Once

	logQueued    atomic.Int64
	logWriteErrs atomic.Int64
	// logWriterIdle is false while rows are queued or buffered, and becomes
	// true once the writer has persisted everything it had. Readers that need
	// to see recent rows (tests, diagnostics) can wait on it.
	logWriterIdle atomic.Bool
)

// LogQueueStats reports the pending entries and the number of rows the writer
// gave up on. Both are exposed for /api/status style diagnostics.
func LogQueueStats() (pending int64, writeErrors int64) {
	return logQueued.Load(), logWriteErrs.Load()
}

// WaitLogQueueDrained blocks until every row enqueued before the call has been
// written, or the timeout elapses. Unlike FlushLogQueue it leaves the writer
// running, so it can be called at any time.
func WaitLogQueueDrained(timeout time.Duration) bool {
	if logQueue == nil {
		return true
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if logWriterIdle.Load() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return logWriterIdle.Load()
}

func startLogQueue() {
	logQueueOnce.Do(func() {
		logQueue = make(chan *Log, logQueueCapacity)
		logStop = make(chan struct{})
		logDrained = make(chan struct{})
		logWorkerWG.Add(1)
		go logQueueWorker()
	})
}

// EnqueueLog hands a log row to the background writer.
//
// The row is never dropped by this path: when the queue is full, or after the
// queue has been drained for shutdown, it is written inline so a burst can at
// worst slow the caller down.
func EnqueueLog(entry *Log) error {
	if entry == nil {
		return nil
	}
	startLogQueue()
	if logStopped.Load() {
		return createLog(entry)
	}
	logWriterIdle.Store(false)
	select {
	case logQueue <- entry:
		logQueued.Add(1)
		return nil
	default:
		// Backpressure: the writer is behind, so absorb the delay here rather
		// than losing a reconciliation row.
		return createLog(entry)
	}
}

func logQueueWorker() {
	defer logWorkerWG.Done()
	defer close(logDrained)

	batch := make([]*Log, 0, logBatchSize)
	ticker := time.NewTicker(logFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		writeLogBatch(batch)
		batch = batch[:0]
	}

	for {
		select {
		case entry := <-logQueue:
			logQueued.Add(-1)
			batch = append(batch, entry)
			if len(batch) >= logBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
			// Nothing queued and nothing buffered: everything enqueued so far
			// is in the database.
			if len(logQueue) == 0 && len(batch) == 0 {
				logWriterIdle.Store(true)
			}
		case <-logStop:
			// Drain whatever is still buffered before the process exits.
			for {
				select {
				case entry := <-logQueue:
					logQueued.Add(-1)
					batch = append(batch, entry)
					if len(batch) >= logBatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

func writeLogBatch(batch []*Log) {
	if len(batch) == 1 {
		writeLogRow(batch[0])
		return
	}
	var err error
	for attempt := 0; attempt < logWriteAttempts; attempt++ {
		err = LOG_DB.CreateInBatches(batch, logBatchSize).Error
		if err == nil {
			return
		}
		common.SysError(common.LogText("failed to write log batch (attempt %d/%d): %s", attempt+1, logWriteAttempts, err.Error()))
		if attempt < logWriteAttempts-1 {
			time.Sleep(logWriteRetryWait)
		}
	}
	// The batch may have failed because of one bad row. Retry row by row so the
	// rest still land.
	for _, entry := range batch {
		writeLogRow(entry)
	}
}

func writeLogRow(entry *Log) {
	var err error
	for attempt := 0; attempt < logWriteAttempts; attempt++ {
		if err = createLog(entry); err == nil {
			return
		}
		if attempt < logWriteAttempts-1 {
			time.Sleep(logWriteRetryWait)
		}
	}
	logWriteErrs.Add(1)
	common.SysError(common.LogText("failed to write log row after %d attempts, row lost: user_id=%d channel_id=%d request_id=%s: %s",
		logWriteAttempts, entry.UserId, entry.ChannelId, entry.RequestId, err.Error()))
}

// FlushLogQueue drains the background writer. Call it once during shutdown,
// before the process exits, so buffered rows are persisted. The wait is capped
// so a database that keeps rejecting writes cannot hang the shutdown path.
func FlushLogQueue() {
	logFlushOnce.Do(func() {
		if logQueue == nil {
			return
		}
		logStopped.Store(true)
		close(logStop)
		select {
		case <-logDrained:
		case <-time.After(10 * time.Second):
			common.SysError(common.LogText("log queue drain timed out, %d rows pending", logQueued.Load()))
		}
	})
}
