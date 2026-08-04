package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildRankedUsers guards the public user-leaderboard row builder:
// DisplayName wins over Username, the chosen name is always masked before it
// leaves the API, ranks are assigned in order, and the slice is never nil.
func TestBuildRankedUsers(t *testing.T) {
	t.Run("display name preferred and masked", func(t *testing.T) {
		rows := buildRankedUsers([]model.RankingUserTotal{
			{UserID: 1, Username: "alice", DisplayName: "Alice", TotalTokens: 300},
			{UserID: 2, Username: "13800138000", TotalTokens: 400},
			{UserID: 3, Username: "zhang123@x.com", TotalTokens: 200},
		})

		require.Len(t, rows, 3)
		assert.Equal(t, 1, rows[0].Rank)
		assert.Equal(t, "Alice", rows[0].DisplayName)
		assert.Equal(t, int64(300), rows[0].TotalTokens)

		assert.Equal(t, 2, rows[1].Rank)
		assert.Equal(t, "138****8000", rows[1].DisplayName)
		assert.Equal(t, int64(400), rows[1].TotalTokens)

		assert.Equal(t, 3, rows[2].Rank)
		assert.Equal(t, "z***3@x.com", rows[2].DisplayName)
		assert.Equal(t, int64(200), rows[2].TotalTokens)
	})

	t.Run("empty input yields empty non-nil slice", func(t *testing.T) {
		rows := buildRankedUsers(nil)
		assert.NotNil(t, rows)
		assert.Empty(t, rows)
	})
}
