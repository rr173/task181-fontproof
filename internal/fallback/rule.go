// Package fallback 实现回退规则的选择、验证与冲突检测。
//
// 覆盖规则以 priority 排序（小者优先）；每条规则绑定按 rank 排序的字体集合，
// 形成字体回退链。本包负责：必需脚本覆盖判定、同一优先级互斥检查、
// 回退循环检测与规则集 checksum（供重排版本冲突判断）。
package fallback

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/typeface"
)

// RuleSnapshot 是规则在验证/发布时的只读视图。
type RuleSnapshot struct {
	Rule         model.FallbackRule
	FontIDs      []string // 按 rank 排序
	Fonts        []model.Font
	RangesByFont map[string][]model.Range
}

// ValidatedRule 是规则验证的结果。
type ValidatedRule struct {
	Status  string   // verifiable / gapped
	Reasons []string // 空洞原因
	Covered bool     // 必需脚本是否全部有字体覆盖
}

// ResolveOrder 返回按 priority 升序、同优先级按名称排序的规则 id 列表。
func ResolveOrder(rules []model.FallbackRule) []string {
	sorted := make([]model.FallbackRule, len(rules))
	copy(sorted, rules)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority == sorted[j].Priority {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Priority < sorted[j].Priority
	})
	out := make([]string, len(sorted))
	for i, r := range sorted {
		out[i] = r.ID
	}
	return out
}

// ValidateRule 校验一条规则的必需脚本覆盖与字体绑定完整性。
// 任一必需脚本没有任何字体覆盖 → gapped（存在空洞）。
func ValidateRule(snap RuleSnapshot, view *typeface.CoverageView) ValidatedRule {
	var reasons []string
	missing := false
	for _, script := range snap.Rule.RequiredScripts {
		covered := false
		for _, fontID := range snap.FontIDs {
			if view.CoversAnyScript(fontID, script) {
				covered = true
				break
			}
		}
		if !covered {
			missing = true
			reasons = append(reasons, fmt.Sprintf("required script %s has no font coverage", script))
		}
	}
	if len(snap.FontIDs) == 0 {
		missing = true
		reasons = append(reasons, "rule binds no fonts")
	}
	if missing {
		return ValidatedRule{Status: model.RuleGapped, Reasons: reasons}
	}
	return ValidatedRule{Status: model.RuleVerifiable, Reasons: nil}
}

// CheckMutualExclusion 检查同一 priority 的两条规则是否互斥。
// 规则覆盖的字体集合与必需脚本集合都相同 → 互斥，应拒绝。
func CheckMutualExclusion(a, b model.FallbackRule, aFonts, bFonts []string) error {
	if a.Priority != b.Priority {
		return nil
	}
	if sameStringSet(aFonts, bFonts) && sameStringSet(a.RequiredScripts, b.RequiredScripts) {
		return model.E("mutually_exclusive", model.ErrMutuallyExclus,
			fmt.Sprintf("rules %s and %s are mutually exclusive at priority %d", a.ID, b.ID, a.Priority))
	}
	return nil
}

// CheckCycle 检测规则集内是否存在字体回退环。
// 构造节点=字体、边=同规则内 rank 小→大 的有向图，DFS 检测环。
func CheckCycle(ruleFonts map[string][]string) error {
	adj := map[string][]string{}
	for _, fonts := range ruleFonts {
		for i := 0; i+1 < len(fonts); i++ {
			adj[fonts[i]] = append(adj[fonts[i]], fonts[i+1])
		}
	}
	state := map[string]int{} // 0=unvisited 1=visiting 2=done
	var dfs func(node string) bool
	dfs = func(node string) bool {
		state[node] = 1
		for _, next := range adj[node] {
			switch state[next] {
			case 1:
				return true // 环
			case 0:
				if dfs(next) {
					return true
				}
			}
		}
		state[node] = 2
		return false
	}
	for node := range adj {
		if state[node] == 0 {
			if dfs(node) {
				return model.E("cycle", model.ErrCycleDetected, "fallback cycle detected among bound fonts")
			}
		}
	}
	return nil
}

// Checksum 计算规则集 checksum：按 priority+name 排序后对每条规则的
// 必需脚本与字体序列做哈希。两个工程师对同一规则集重排时，若 checksum
// 与最新发布的配置不一致 → 版本冲突。
func Checksum(rules []model.FallbackRule, fontsByRule map[string][]string) string {
	h := sha256.New()
	order := ResolveOrder(rules)
	for _, ruleID := range order {
		var r *model.FallbackRule
		for i := range rules {
			if rules[i].ID == ruleID {
				r = &rules[i]
				break
			}
		}
		if r == nil {
			continue
		}
		fmt.Fprintf(h, "%d|%s|%s|%s;", r.Priority, r.ID, strings.Join(r.RequiredScripts, ","), strings.Join(fontsByRule[r.ID], ","))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ReorderResult 是一次规则重排的执行结果。
type ReorderResult struct {
	Rules    []model.FallbackRule
	Checksum string
	Version  int
}

// ApplyReorder 按新的 priority 序列重排规则并返回新 checksum。
// expectChecksum 为调用方持有的旧 checksum，若与当前不一致返回冲突。
func ApplyReorder(rules []model.FallbackRule, fontsByRule map[string][]string, newOrder []string, expectChecksum string) (*ReorderResult, error) {
	current := Checksum(rules, fontsByRule)
	if expectChecksum != "" && expectChecksum != current {
		return nil, model.E("conflict", model.ErrChecksumMismatch,
			"rule set checksum mismatch: another engineer reordered rules concurrently")
	}
	if len(newOrder) != len(rules) {
		return nil, model.EBadRequest("new order length must equal rule count")
	}
	byID := map[string]*model.FallbackRule{}
	for i := range rules {
		byID[rules[i].ID] = &rules[i]
	}
	if len(newOrder) != len(byID) {
		return nil, model.EBadRequest("new order contains duplicate or unknown rule ids")
	}
	seen := map[string]bool{}
	for i, ruleID := range newOrder {
		r, ok := byID[ruleID]
		if !ok {
			return nil, model.EBadRequest(fmt.Sprintf("unknown rule %s in new order", ruleID))
		}
		if seen[ruleID] {
			return nil, model.EBadRequest(fmt.Sprintf("duplicate rule %s in new order", ruleID))
		}
		seen[ruleID] = true
		r.Priority = i + 1
	}
	newRules := make([]model.FallbackRule, 0, len(rules))
	for _, ruleID := range newOrder {
		newRules = append(newRules, *byID[ruleID])
	}
	return &ReorderResult{
		Rules:    newRules,
		Checksum: Checksum(newRules, fontsByRule),
		Version:  model.NextRuleSetVersion(rules),
	}, nil
}

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
