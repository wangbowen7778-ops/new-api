package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// installLogTestDB swaps LOG_DB for an in-memory database holding only the log
// table and restores it afterwards.
func installLogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previous := LOG_DB
	previousType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.MainDatabaseType(), common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db
	t.Cleanup(func() {
		LOG_DB = previous
		common.SetDatabaseTypes(common.MainDatabaseType(), previousType)
		_ = sqlDB.Close()
	})
	return db
}

// waitLogQueueIdle blocks until the writer has drained the queue and the last
// batch has been flushed.
func waitLogQueueIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if logQueued.Load() == 0 {
			// Give the ticker one more period to flush the final batch.
			time.Sleep(logFlushInterval * 2)
			if logQueued.Load() == 0 {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("log queue did not drain, %d rows still pending", logQueued.Load())
}

func countLogRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var total int64
	require.NoError(t, db.Model(&Log{}).Count(&total).Error)
	return total
}

// Reconciliation depends on every billing/error row landing, so enqueueing must
// never silently drop one.
func TestEnqueueLogPersistsEveryRow(t *testing.T) {
	db := installLogTestDB(t)

	const rows = 500
	for i := 0; i < rows; i++ {
		require.NoError(t, EnqueueLog(&Log{
			UserId:    1,
			Type:      LogTypeConsume,
			Content:   "async log test",
			CreatedAt: common.GetTimestamp(),
		}))
	}
	waitLogQueueIdle(t)

	require.EqualValues(t, rows, countLogRows(t, db), "every enqueued row must reach the database")
}

// A burst larger than the queue capacity must fall back to inline writes rather
// than dropping the overflow.
func TestEnqueueLogOverflowStillPersists(t *testing.T) {
	db := installLogTestDB(t)

	rows := logQueueCapacity + 500
	for i := 0; i < rows; i++ {
		require.NoError(t, EnqueueLog(&Log{
			UserId:    2,
			Type:      LogTypeError,
			Content:   "overflow row",
			CreatedAt: common.GetTimestamp(),
		}))
	}
	waitLogQueueIdle(t)

	require.EqualValues(t, rows, countLogRows(t, db), "overflow rows must be written inline, not dropped")
}

// After shutdown starts, new rows go straight to the database instead of a
// queue nobody drains.
func TestEnqueueLogWritesInlineAfterStop(t *testing.T) {
	db := installLogTestDB(t)

	logStopped.Store(true)
	t.Cleanup(func() { logStopped.Store(false) })

	require.NoError(t, EnqueueLog(&Log{
		UserId:    3,
		Type:      LogTypeError,
		Content:   "after stop",
		CreatedAt: common.GetTimestamp(),
	}))
	require.EqualValues(t, 1, countLogRows(t, db))
}
