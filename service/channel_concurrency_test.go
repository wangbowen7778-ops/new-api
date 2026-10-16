package service

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquireChannelConcurrency(t *testing.T) {
	t.Run("unlimited when limit is zero", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			release, ok := AcquireChannelConcurrency(1001, 0)
			require.True(t, ok)
			release()
		}
	})

	t.Run("rejects when limit reached then recovers after release", func(t *testing.T) {
		release1, ok := AcquireChannelConcurrency(1002, 2)
		require.True(t, ok)
		require.NotNil(t, release1)

		release2, ok := AcquireChannelConcurrency(1002, 2)
		require.True(t, ok)

		_, ok = AcquireChannelConcurrency(1002, 2)
		require.False(t, ok)

		release2()
		release2() // idempotent

		release3, ok := AcquireChannelConcurrency(1002, 2)
		require.True(t, ok)
		release3()
		release1()
	})

	t.Run("independent counters per channel", func(t *testing.T) {
		releaseA, ok := AcquireChannelConcurrency(1003, 1)
		require.True(t, ok)

		_, ok = AcquireChannelConcurrency(1004, 1)
		require.True(t, ok)

		_, ok = AcquireChannelConcurrency(1003, 1)
		require.False(t, ok)

		releaseA()
	})

	t.Run("concurrent acquires respect the limit", func(t *testing.T) {
		const limit = 5
		var mu sync.Mutex
		acquired := 0
		rejected := 0

		releases := make([]func(), 0, 20)
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				release, ok := AcquireChannelConcurrency(1005, limit)
				mu.Lock()
				defer mu.Unlock()
				if ok {
					acquired++
					releases = append(releases, release)
				} else {
					rejected++
				}
			}()
		}
		wg.Wait()

		assert.Equal(t, limit, acquired)
		assert.Equal(t, 20-limit, rejected)

		for _, release := range releases {
			release()
		}
		_, ok := AcquireChannelConcurrency(1005, limit)
		require.True(t, ok)
	})
}
