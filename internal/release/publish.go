package release

import (
	"fmt"

	"task181-fontproof/internal/model"
)

// PublishInput 是发布配置的请求载荷。
type PublishInput struct {
	Name string `json:"name"`
}

// PublishResult 是一次发布的结果。
type PublishResult struct {
	Config   model.PublishedConfig
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
		ID:       id,
		ConfigID: id,
		Version:  version,
		Checksum: checksum,
		Snapshot: snapshot,
	}, nil
}

// SnapshotEqual 判断两个快照是否等价：规则与字体按标识组成集合比较，
// 集合顺序变化不影响等价判断（用于配置比较的粗判）。
func SnapshotEqual(a, b *Snapshot) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.RuleVersion != b.RuleVersion {
		return false
	}
	if len(a.Rules) != len(b.Rules) || len(a.Fonts) != len(b.Fonts) {
		return false
	}
	// 规则按 rule_id 组成集合比较（含 priority、绑定字体、脚本）
	baseRules := map[string]FrozenRule{}
	for _, r := range a.Rules {
		baseRules[r.RuleID] = r
	}
	for _, r := range b.Rules {
		br, ok := baseRules[r.RuleID]
		if !ok || !frozenRuleEqual(br, r) {
			return false
		}
	}
	// 字体按 font_id 组成集合比较
	baseFonts := map[string]FrozenFont{}
	for _, f := range a.Fonts {
		baseFonts[f.FontID] = f
	}
	for _, f := range b.Fonts {
		bf, ok := baseFonts[f.FontID]
		if !ok || bf != f {
			return false
		}
	}
	return true
}

// frozenRuleEqual 比较两条冻结规则的字段（切片按集合比较，顺序无关）。
func frozenRuleEqual(a, b FrozenRule) bool {
	if a.RuleID != b.RuleID || a.Name != b.Name || a.Priority != b.Priority {
		return false
	}
	return sameStringSet(a.FontIDs, b.FontIDs) && sameStringSet(a.Scripts, b.Scripts)
}

// sameStringSet 判断两个字符串切片是否包含相同元素（顺序无关）。
func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]bool{}
	for _, s := range a {
		m[s] = true
	}
	for _, s := range b {
		if !m[s] {
			return false
		}
	}
	return true
}
