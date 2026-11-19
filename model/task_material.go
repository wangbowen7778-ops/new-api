package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// TaskMaterial 记录一次素材（人物资产）上传落在哪个渠道上游，以及本地副本位置。
//
// 上游的资产 ID（material.asset.xxx）只在上传它的那个调用身份下有效，换渠道
// 或换 Key 再引用会认不出来。所以每次上传都把渠道绑定关系记下来：查询走原渠道，
// 生成时校验资产与当前渠道是否一致，不一致就明确报错，而不是发出去被上游拒。
type TaskMaterial struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	MaterialId   string `json:"material_id" gorm:"type:varchar(191);uniqueIndex"`
	Object       string `json:"object" gorm:"type:varchar(32)"`
	Model        string `json:"model" gorm:"type:varchar(191);index"`
	MaterialType string `json:"material_type" gorm:"type:varchar(32)"`
	// Kind/Token/Seconds 只在共享车素材（/v1/fleet/media）上有值：
	// kind 是 image/video/audio/file，token 是素材短名（@图片1 用的那个），
	// seconds 是音视频时长。
	Kind      string  `json:"kind" gorm:"type:varchar(16)"`
	Token     string  `json:"token" gorm:"type:varchar(64)"`
	Seconds   float64 `json:"seconds"`
	UserId    int     `json:"user_id" gorm:"index"`
	TokenId   int     `json:"token_id" gorm:"index"`
	ChannelId int     `json:"channel_id" gorm:"index"`
	BaseUrl   string  `json:"base_url" gorm:"type:varchar(255)"`
	SourceUrl string  `json:"source_url" gorm:"type:text"`
	LocalPath string  `json:"local_path" gorm:"type:varchar(255)"`
	Size      int64   `json:"size"`
	Status    string  `json:"status" gorm:"type:varchar(32)"`
	ErrorCode string  `json:"error_code" gorm:"type:varchar(64)"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

func (TaskMaterial) TableName() string {
	return "task_materials"
}

// GetTaskMaterialByMaterialId 查不到时返回 (nil, nil)，调用方按“未知资产”处理。
func GetTaskMaterialByMaterialId(materialId string) (*TaskMaterial, error) {
	if materialId == "" {
		return nil, nil
	}
	var material TaskMaterial
	err := DB.Where("material_id = ?", materialId).First(&material).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &material, nil
}

// GetLatestTaskMaterialByToken 取同一用户最近一次上传的同名素材，用于让
// 共享车素材固定落在同一个渠道上（素材只能由上传它的调用身份使用）。
func GetLatestTaskMaterialByToken(token, kind string, userId int) (*TaskMaterial, error) {
	if token == "" || kind == "" {
		return nil, nil
	}
	var material TaskMaterial
	err := DB.Where("token = ? AND kind = ? AND user_id = ?", token, kind, userId).
		Order("id DESC").First(&material).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &material, nil
}

// UpsertTaskMaterial 按 material_id 幂等写入：重复查询同一个资产只更新状态。
func UpsertTaskMaterial(material *TaskMaterial) error {
	if material == nil || material.MaterialId == "" {
		return nil
	}
	material.UpdatedAt = common.GetTimestamp()
	existing, err := GetTaskMaterialByMaterialId(material.MaterialId)
	if err != nil {
		return err
	}
	if existing == nil {
		if material.CreatedAt == 0 {
			material.CreatedAt = material.UpdatedAt
		}
		return DB.Create(material).Error
	}
	material.Id = existing.Id
	if material.CreatedAt == 0 {
		material.CreatedAt = existing.CreatedAt
	}
	if material.LocalPath == "" {
		material.LocalPath = existing.LocalPath
	}
	if material.SourceUrl == "" {
		material.SourceUrl = existing.SourceUrl
	}
	if material.Size == 0 {
		material.Size = existing.Size
	}
	if material.Kind == "" {
		material.Kind = existing.Kind
	}
	if material.Token == "" {
		material.Token = existing.Token
	}
	return DB.Save(material).Error
}
