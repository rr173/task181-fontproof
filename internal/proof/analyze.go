package proof

import (
	"task181-fontproof/internal/grapheme"
	"task181-fontproof/internal/model"
)

// Analyze 对样本文本执行完整覆盖分析：切分字素簇 → 逐簇证明 → 汇总统计。
// 返回字素簇结果、统计与错误（幂等：不依赖外部状态）。
func Analyze(e *Engine, text string) ([]model.Grapheme, model.AnalysisStats, error) {
	clusters := grapheme.Segmenter(text)
	if len(clusters) == 0 {
		return nil, model.AnalysisStats{}, model.EBadRequest("sample text contains no grapheme clusters")
	}
	out := make([]model.Grapheme, 0, len(clusters))
	stats := model.AnalysisStats{Total: len(clusters)}
	for idx, cl := range clusters {
		res := e.ProveCluster(cl.Codepoints, cl.IsComposite, int(cl.Kind))
		if res.Status == model.ClusterRisk && res.Reason == "" {
			res.Reason = grapheme.SplitReason(cl.Kind)
		}
		g := model.Grapheme{
			Position:    idx,
			Text:        cl.Text,
			Codepoints:  cl.Codepoints,
			Script:      cl.Script,
			Status:      res.Status,
			Chain:       res.Chain,
			RiskReason:  res.Reason,
			IsComposite: cl.IsComposite,
		}
		switch res.Status {
		case model.ClusterCovered:
			stats.Covered++
		case model.ClusterFallback:
			stats.Fallback++
		case model.ClusterRisk:
			stats.Risk++
		case model.ClusterMissing:
			stats.Missing++
		}
		out = append(out, g)
	}
	return out, stats, nil
}

// RiskClusters 返回拆分风险字素簇（含原因与回退链）。
func RiskClusters(gs []model.Grapheme) []model.Grapheme {
	var out []model.Grapheme
	for _, g := range gs {
		if g.Status == model.ClusterRisk {
			out = append(out, g)
		}
	}
	return out
}

// MissingChains 返回缺字字素簇及其最短缺失链证明。
func MissingChains(gs []model.Grapheme) []model.Grapheme {
	var out []model.Grapheme
	for _, g := range gs {
		if g.Status == model.ClusterMissing {
			out = append(out, g)
		}
	}
	return out
}

// Passed 判断分析是否无缺字且无拆分风险。
func Passed(stats model.AnalysisStats) bool {
	return stats.Missing == 0 && stats.Risk == 0
}

// Summarize 生成统计摘要文本。
func Summarize(stats model.AnalysisStats) string {
	return "total=" + itoa(stats.Total) + " covered=" + itoa(stats.Covered) +
		" fallback=" + itoa(stats.Fallback) + " risk=" + itoa(stats.Risk) +
		" missing=" + itoa(stats.Missing)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
