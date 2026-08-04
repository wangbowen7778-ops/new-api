package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetRankingUserTotals guards the aggregation + LEFT JOIN used by the
// public user leaderboard: correct ordering, time-range filter, limit, and the
// users.display_name join (including rows whose user no longer exists).
func TestGetRankingUserTotals(t *testing.T) {
	truncateTables(t)

	alice := &User{Username: "alice", DisplayName: "Alice", AffCode: "aff_alice"}
	bob := &User{Username: "bob", AffCode: "aff_bob"}
	carol := &User{Username: "13800138000", AffCode: "aff_carol"}
	require.NoError(t, DB.Create(alice).Error)
	require.NoError(t, DB.Create(bob).Error)
	require.NoError(t, DB.Create(carol).Error)

	const now = int64(1700000000)
	quotaRows := []QuotaData{
		// alice aggregates two models: 100 + 200 = 300 tokens.
		{UserID: alice.Id, Username: "alice", ModelName: "m1", CreatedAt: now, TokenUsed: 100},
		{UserID: alice.Id, Username: "alice", ModelName: "m2", CreatedAt: now, TokenUsed: 200},
		{UserID: bob.Id, Username: "bob", ModelName: "m1", CreatedAt: now, TokenUsed: 50},
		{UserID: carol.Id, Username: "13800138000", ModelName: "m1", CreatedAt: now, TokenUsed: 400},
		// Outside the queried time range -> must be excluded.
		{UserID: bob.Id, Username: "bob", ModelName: "m2", CreatedAt: now + 4000, TokenUsed: 999},
		// Deleted/never-existing user: LEFT JOIN must keep the row with empty display_name.
		{UserID: 999999, Username: "ghostuser", ModelName: "m1", CreatedAt: now, TokenUsed: 70},
	}
	for i := range quotaRows {
		require.NoError(t, DB.Create(&quotaRows[i]).Error)
	}

	got, err := GetRankingUserTotals(now-1, now+1, 5)
	require.NoError(t, err)
	require.Len(t, got, 4)

	assert.Equal(t, carol.Id, got[0].UserID)
	assert.Equal(t, int64(400), got[0].TotalTokens)
	assert.Equal(t, "13800138000", got[0].Username)
	assert.Equal(t, "", got[0].DisplayName)

	assert.Equal(t, alice.Id, got[1].UserID)
	assert.Equal(t, int64(300), got[1].TotalTokens)
	assert.Equal(t, "alice", got[1].Username)
	assert.Equal(t, "Alice", got[1].DisplayName)

	assert.Equal(t, 999999, got[2].UserID)
	assert.Equal(t, int64(70), got[2].TotalTokens)
	assert.Equal(t, "ghostuser", got[2].Username)
	assert.Equal(t, "", got[2].DisplayName)

	assert.Equal(t, bob.Id, got[3].UserID)
	assert.Equal(t, int64(50), got[3].TotalTokens)

	limited, err := GetRankingUserTotals(now-1, now+1, 2)
	require.NoError(t, err)
	require.Len(t, limited, 2)
	assert.Equal(t, carol.Id, limited[0].UserID)
	assert.Equal(t, alice.Id, limited[1].UserID)
}
