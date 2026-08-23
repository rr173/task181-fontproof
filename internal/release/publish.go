package release

import (
	"encoding/json"
	"fmt"

	"task181-fontproof/internal/model"
)

// PublishInput 是发布配置的请求载荷。
type PublishInput struct {
	Name string `json:"name"`
}

// PublishResult 是一次发布的结果。
type PublishResult struct {
	Config  model.PublishedConfig
	Snapshot *Snapshot
}

// NewPublishedConfig 构造新的已发布配置（由 store 负责持久化）。
func NewPublishedConfig(id, name string, ruleVersion int, checksum string, snap *Snapshot) (*PublishResult, error) {
	data, err := snap.Marshal()
	if err != nil {
		return nil, model.EBadRequest("failed to serialize config snapshot: " + err.Error())
	}
	return &PublishResult{
		Config: model.PublishedConfig{
			ID:          id,
			Name:        name,
			RuleVersion: ruleVersion,
			Checksum:    checksum,
			Status:      model.ConfigActive,
			Snapshot:    data,
		},
		Snapshot: snap,
	}, nil
}

// ValidateSupersede 校验对已发布配置的替代操作：只有 active 配置可被替代。
func ValidateSupersede(cfg model.PublishedConfig) error {
	if cfg.Status != model.ConfigActive {
		return model.EInvalidState(fmt.Sprintf("config %s is not active, cannot supersede", cfg.ID))
	}
	return nil
}

// AppendConfigVersion 构造配置版本历史条目。
func AppendConfigVersion(id string, version int, checksum, snapshot string) (model.ConfigVersion, error) {
	return model.ConfigVersion{
		ID:        id,
		ConfigID:  id,
		Version:   version,
		Checksum:  checksum,
		Snapshot:  snapshot,
	}, nil
}

// SnapshotEqual 判断两个快照的规则部分是否等价（用于配置比较的粗判）。
func SnapshotEqual(a, b *Snapshot) bool {
	if a.RuleVersion != b.RuleVersion || len(a.Rules) != len(b.Rules) || len(a.Fonts) != len(b.Fonts) {
		return false
	}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}
