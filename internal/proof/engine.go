// Package proof 实现覆盖证明引擎：字素簇 → 字体选择链 → 最短缺失链与拆分风险。
//
// 对每个字素簇，按回退规则的字体优先级逐个检查覆盖；能覆盖所有码点的
// 最小前缀字体序列即“字体选择链”。若所有字体仍无法覆盖某些码点，则输出
// 最短缺失链（覆盖最多码点的最短序列）与缺失码点证明。
package proof

import (
	"task181-fontproof/internal/model"
	"task181-fontproof/internal/typeface"
)

// Engine 是覆盖证明引擎。
type Engine struct {
	View        *typeface.CoverageView
	Order       []string            // 规则 id 列表（按 priority 升序）
	FontsByRule map[string][]string // 规则 → 字体序列（按 rank）
}

// ClusterResult 是单个字素簇的证明结果。
type ClusterResult struct {
	Status  string
	Chain   []string // 字体选择链（font id）
	Reason  string   // 拆分风险或缺字原因
	Missing []rune   // 缺失码点（缺字时非空）
}

// ProveCluster 对单个字素簇执行覆盖证明。
func (e *Engine) ProveCluster(cps []rune, composite bool, clusterKind int) ClusterResult {
	// 1. 按规则优先级聚合字体顺序（去重，保留首次出现位置）。
	fontOrder := e.fontOrder()

	// 2. 贪心构建选择链：依次取字体，覆盖任一未覆盖码点则加入链并标记。
	var chain []string
	covered := map[rune]bool{}
	for _, fontID := range fontOrder {
		if !e.CoversAnyOf(fontID, cps) {
			continue
		}
		if markNewlyCovered(fontID, cps, covered, e.View) {
			chain = append(chain, fontID)
		}
		if allCovered(cps, covered) {
			break
		}
	}
	missing := missingSet(cps, covered)

	// 3. 判定状态。
	switch {
	case len(missing) == 0 && len(chain) == 0:
		// 无字体可覆盖但无缺失（理论不可能，防御处理）
		return ClusterResult{Status: model.ClusterMissing, Reason: "no font available", Missing: missing}
	case len(missing) == 0 && len(chain) == 1:
		return ClusterResult{Status: model.ClusterCovered, Chain: chain}
	case len(missing) == 0:
		// 多字体链：复合簇 → 拆分风险
		if composite {
			return ClusterResult{Status: model.ClusterRisk, Chain: chain, Reason: splitReason(clusterKind)}
		}
		return ClusterResult{Status: model.ClusterFallback, Chain: chain}
	default:
		return ClusterResult{Status: model.ClusterMissing, Chain: chain, Missing: missing, Reason: missingReason(missing)}
	}
}

// fontOrder 聚合所有规则的字体序列，去重保序。
func (e *Engine) fontOrder() []string {
	seen := map[string]bool{}
	var out []string
	for _, ruleID := range e.Order {
		for _, fontID := range e.FontsByRule[ruleID] {
			if !seen[fontID] {
				seen[fontID] = true
				out = append(out, fontID)
			}
		}
	}
	return out
}

// ShortestMissingProof 计算最短缺失链：对每个缺失码点，找到按优先级
// 能覆盖它的字体集合；返回“覆盖最多码点”的最短字体序列证明。
// 这里以贪心前缀为准（与 ProveCluster 同语义），保证证明可复现。
func (e *Engine) ShortestMissingProof(cps []rune) (chain []string, missing []rune) {
	fontOrder := e.fontOrder()
	covered := map[rune]bool{}
	for _, fontID := range fontOrder {
		if !e.CoversAnyOf(fontID, cps) {
			continue
		}
		if markNewlyCovered(fontID, cps, covered, e.View) {
			chain = append(chain, fontID)
		}
		if allCovered(cps, covered) {
			break
		}
	}
	return chain, missingSet(cps, covered)
}

func (e *Engine) CoversAnyOf(fontID string, cps []rune) bool {
	for _, cp := range cps {
		if e.View.Covers(fontID, cp) {
			return true
		}
	}
	return false
}

// markNewlyCovered 将字体能覆盖的未覆盖码点标记为已覆盖；返回是否新增覆盖。
func markNewlyCovered(fontID string, cps []rune, covered map[rune]bool, view *typeface.CoverageView) bool {
	added := false
	for _, cp := range cps {
		if !covered[cp] && view.Covers(fontID, cp) {
			covered[cp] = true
			added = true
		}
	}
	return added
}

func allCovered(cps []rune, covered map[rune]bool) bool {
	for _, cp := range cps {
		if !covered[cp] {
			return false
		}
	}
	return true
}

func missingSet(cps []rune, covered map[rune]bool) []rune {
	var out []rune
	seen := map[rune]bool{}
	for _, cp := range cps {
		if !covered[cp] && !seen[cp] {
			seen[cp] = true
			out = append(out, cp)
		}
	}
	return out
}

func splitReason(kind int) string {
	switch kind {
	case 2: // KindCombining
		return "variation selector split from base character"
	case 3: // KindVariation
		return "variation selector split from base character"
	case 4: // KindZWJ
		return "ZWJ sequence split across fonts"
	case 5: // KindModifier
		return "emoji modifier split from base emoji"
	case 6: // KindNumeric
		return "numeric sequence split across fonts (style inconsistency risk)"
	}
	return "composite grapheme split across fonts"
}

func missingReason(missing []rune) string {
	s := "missing codepoints not covered by any font:"
	for _, cp := range missing {
		s += " U+" + hex4(cp)
	}
	return s
}

func hex4(cp rune) string {
	const digits = "0123456789ABCDEF"
	var buf [4]byte
	for i := 3; i >= 0; i-- {
		buf[i] = digits[cp&0xF]
		cp >>= 4
	}
	return string(buf[:])
}
