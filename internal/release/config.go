// Package release 实现回退配置的发布冻结、版本历史与版本比较。
//
// 发布配置在创建时冻结规则快照与字体覆盖摘要；已发布配置不可改写，
// 只能被新的发布替代（supersede）。版本比较输出规则增删、重排与字体变化。
package release

import (
	"encoding/json"
	"fmt"
	"sort"

	"task181-fontproof/internal/model"
)

// FrozenRule 是快照中的规则条目。
type FrozenRule struct {
	RuleID   string   `json:"rule_id"`
	Name     string   `json:"name"`
	Priority int      `json:"priority"`
	FontIDs  []string `json:"font_ids"`
	Scripts  []string `json:"scripts"`
}

// FrozenFont 是快照中的字体覆盖摘要条目。
type FrozenFont struct {
	FontID      string `json:"font_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Ranges      string `json:"ranges"` // 人类可读区间
	Fingerprint string `json:"fingerprint"`
}

// Snapshot 是冻结配置的完整内容。
type Snapshot struct {
	RuleVersion int          `json:"rule_version"`
	Rules       []FrozenRule `json:"rules"`
	Fonts       []FrozenFont `json:"fonts"`
}

// CanonicalizeSnapshot returns a deep, deterministic copy whose unordered
// snapshot collections are sorted by immutable IDs. It never mutates a caller's
// snapshot, which is important when comparing a live publish request.
func CanonicalizeSnapshot(in *Snapshot) *Snapshot {
	if in == nil {
		return nil
	}
	out := &Snapshot{RuleVersion: in.RuleVersion}
	out.Rules = append([]FrozenRule(nil), in.Rules...)
	out.Fonts = append([]FrozenFont(nil), in.Fonts...)
	for i := range out.Rules {
		out.Rules[i].FontIDs = append([]string(nil), out.Rules[i].FontIDs...)
		out.Rules[i].Scripts = append([]string(nil), out.Rules[i].Scripts...)
		sort.Strings(out.Rules[i].FontIDs)
		sort.Strings(out.Rules[i].Scripts)
	}
	sort.Slice(out.Rules, func(i, j int) bool { return out.Rules[i].RuleID < out.Rules[j].RuleID })
	sort.Slice(out.Fonts, func(i, j int) bool { return out.Fonts[i].FontID < out.Fonts[j].FontID })
	return out
}

// BuildSnapshot 从当前规则与字体状态构建冻结快照。
func BuildSnapshot(ruleVersion int, rules []model.FallbackRule, fontsByRule map[string][]string,
	fonts []model.Font, rangeDesc map[string]string) (*Snapshot, error) {
	snap := &Snapshot{RuleVersion: ruleVersion}
	for _, r := range rules {
		snap.Rules = append(snap.Rules, FrozenRule{
			RuleID:   r.ID,
			Name:     r.Name,
			Priority: r.Priority,
			FontIDs:  append([]string(nil), fontsByRule[r.ID]...),
			Scripts:  append([]string(nil), r.RequiredScripts...),
		})
	}
	for _, f := range fonts {
		snap.Fonts = append(snap.Fonts, FrozenFont{
			FontID:      f.ID,
			Name:        f.Name,
			Status:      f.Status,
			Ranges:      rangeDesc[f.ID],
			Fingerprint: f.Fingerprint,
		})
	}
	return snap, nil
}

// Marshal 将快照序列化为 JSON 字符串。
func (s *Snapshot) Marshal() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Unmarshal 从 JSON 字符串解析快照。
func Unmarshal(data string) (*Snapshot, error) {
	var s Snapshot
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Compare 比较两个配置快照，输出差异摘要。
func Compare(base, target *Snapshot) model.ConfigDiff {
	diff := model.ConfigDiff{
		BaseID:   fmt.Sprintf("v%d", base.RuleVersion),
		TargetID: fmt.Sprintf("v%d", target.RuleVersion),
	}
	baseRules := map[string]FrozenRule{}
	for _, r := range base.Rules {
		baseRules[r.RuleID] = r
	}
	targetRules := map[string]FrozenRule{}
	for _, r := range target.Rules {
		targetRules[r.RuleID] = r
	}
	for id, r := range targetRules {
		if _, ok := baseRules[id]; !ok {
			diff.AddedRules = append(diff.AddedRules, r.Name)
		}
	}
	for id, r := range baseRules {
		if _, ok := targetRules[id]; !ok {
			diff.RemovedRules = append(diff.RemovedRules, r.Name)
		}
	}
	// 重排检测：同一规则集合下优先级顺序变化
	if len(diff.AddedRules) == 0 && len(diff.RemovedRules) == 0 {
		baseOrder := priorityOrder(baseRules)
		targetOrder := priorityOrder(targetRules)
		for i := range baseOrder {
			if i >= len(targetOrder) || baseOrder[i] != targetOrder[i] {
				diff.Reordered = true
				break
			}
		}
	}
	// 字体变化检测
	baseFonts := map[string]FrozenFont{}
	for _, f := range base.Fonts {
		baseFonts[f.FontID] = f
	}
	for _, f := range target.Fonts {
		if bf, ok := baseFonts[f.FontID]; ok {
			if bf.Ranges != f.Ranges || bf.Status != f.Status {
				diff.ChangedFonts = append(diff.ChangedFonts, f.Name)
			}
		} else {
			diff.ChangedFonts = append(diff.ChangedFonts, f.Name)
		}
	}
	diff.Same = len(diff.AddedRules) == 0 && len(diff.RemovedRules) == 0 &&
		!diff.Reordered && len(diff.ChangedFonts) == 0
	if diff.Same {
		diff.Summary = "configurations are identical"
	} else {
		diff.Summary = fmt.Sprintf("added %d, removed %d, reordered %v, changed fonts %d",
			len(diff.AddedRules), len(diff.RemovedRules), diff.Reordered, len(diff.ChangedFonts))
	}
	return diff
}

// priorityOrder 按 priority 升序返回规则 id 序列。
func priorityOrder(rules map[string]FrozenRule) []string {
	ids := make([]string, 0, len(rules))
	for id := range rules {
		ids = append(ids, id)
	}
	// 冒泡按 priority 排序（快照规模小）
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if rules[ids[j]].Priority < rules[ids[i]].Priority {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}
