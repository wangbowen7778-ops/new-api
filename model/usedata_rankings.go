package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type RankingQuotaTotal struct {
	ModelName   string `json:"model_name"`
	TotalTokens int64  `json:"total_tokens"`
}

type RankingQuotaBucket struct {
	ModelName string `json:"model_name"`
	Bucket    int64  `json:"bucket"`
	Tokens    int64  `json:"tokens"`
}

// RankingUserTotal aggregates a single user's token usage for the rankings page.
type RankingUserTotal struct {
	UserID      int    `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	TotalTokens int64  `json:"total_tokens"`
}

func GetRankingQuotaTotals(startTime int64, endTime int64) ([]RankingQuotaTotal, error) {
	var rows []RankingQuotaTotal
	query := DB.Table("quota_data").
		Select("model_name, sum(token_used) as total_tokens").
		Where("model_name <> ''").
		Group("model_name").
		Having("sum(token_used) > 0").
		Order("total_tokens DESC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}

func GetRankingQuotaBuckets(startTime int64, endTime int64, bucketSize int64) ([]RankingQuotaBucket, error) {
	if bucketSize <= 0 {
		bucketSize = 3600
	}
	bucketExpr := rankingBucketExpr(bucketSize)
	var rows []RankingQuotaBucket
	query := DB.Table("quota_data").
		Select(fmt.Sprintf("model_name, %s as bucket, sum(token_used) as tokens", bucketExpr)).
		Where("model_name <> ''").
		Group(fmt.Sprintf("model_name, %s", bucketExpr)).
		Having("sum(token_used) > 0").
		Order("bucket ASC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}

func rankingBucketExpr(bucketSize int64) string {
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		return fmt.Sprintf("FLOOR(created_at / %d) * %d", bucketSize, bucketSize)
	}
	return fmt.Sprintf("(created_at / %d) * %d", bucketSize, bucketSize)
}

func applyRankingQuotaTimeRange(query *gorm.DB, startTime int64, endTime int64) *gorm.DB {
	if startTime > 0 {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_at <= ?", endTime)
	}
	return query
}

// GetRankingUserTotals returns the top `limit` users by token usage within
// [startTime, endTime]. Admin users are intentionally NOT excluded. The LEFT
// JOIN keeps quota_data rows whose user was deleted or never existed
// (display_name is NULL there -> scanned as "").
func GetRankingUserTotals(startTime int64, endTime int64, limit int) ([]RankingUserTotal, error) {
	if limit <= 0 {
		limit = 5
	}
	var rows []RankingUserTotal
	query := DB.Table("quota_data").
		Select("quota_data.user_id, quota_data.username, users.display_name, sum(quota_data.token_used) as total_tokens").
		Joins("left join users on users.id = quota_data.user_id").
		Group("quota_data.user_id, quota_data.username, users.display_name").
		Having("sum(quota_data.token_used) > 0").
		Order("total_tokens DESC, quota_data.user_id ASC").
		Limit(limit)
	// Qualify created_at with quota_data: users also has a created_at column
	// (GORM auto-timestamps), so the shared applyRankingQuotaTimeRange would be
	// ambiguous once users is joined.
	if startTime > 0 {
		query = query.Where("quota_data.created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("quota_data.created_at <= ?", endTime)
	}
	err := query.Find(&rows).Error
	return rows, err
}
