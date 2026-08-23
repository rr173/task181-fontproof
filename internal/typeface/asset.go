package typeface

import (
	"task181-fontproof/internal/model"
)

// 字体资产状态机迁移表：from → 允许的 to 集合。
var fontTransitions = map[string]map[string]bool{
	model.FontPendingScan: {
		model.FontAvailable: true,
		model.FontConflict:  true,
	},
	model.FontAvailable: {
		model.FontDisabled: true,
		model.FontConflict: true,
	},
	model.FontConflict: {
		model.FontAvailable: true,
		model.FontDisabled: true,
	},
	model.FontDisabled: {
		model.FontAvailable: true,
	},
}

// CanTransition 判断字体资产是否允许 from→to 状态迁移。
func CanTransition(from, to string) bool {
	if tos, ok := fontTransitions[from]; ok {
		return tos[to]
	}
	return false
}

// ScanOutcome 是一次扫描的结果：根据覆盖范围与冲突判定可用或冲突。
type ScanOutcome struct {
	Status     string
	Reason     string
	Total      int
	HasConflict bool
}

// EvaluateScan 评估字体扫描结果。
// - 空覆盖 → 冲突（无码点）
// - 覆盖区间与既有字体无冲突且非空 → available
// - 存在明显重叠冲突标记 → conflict
func EvaluateScan(total int, conflict bool) ScanOutcome {
	if total <= 0 {
		return ScanOutcome{Status: model.FontConflict, Reason: "font covers no codepoints", Total: total, HasConflict: true}
	}
	if conflict {
		return ScanOutcome{Status: model.FontConflict, Reason: "overlapping coverage conflict detected", Total: total, HasConflict: true}
	}
	return ScanOutcome{Status: model.FontAvailable, Reason: "scan ok", Total: total}
}

// DetectCoverageConflict 检测两份字体覆盖区间是否存在“实质性重叠”。
// 阈值：重叠码点数 ≥ 16 视为实质性重叠（同一脚本字体的正常局部重叠不算）。
func DetectCoverageConflict(a, b []model.Range) bool {
	overlap := 0
	for _, ra := range a {
		for _, rb := range b {
			lo := ra.Start
			if rb.Start > lo {
				lo = rb.Start
			}
			hi := ra.End
			if rb.End < hi {
				hi = rb.End
			}
			if lo <= hi {
				overlap += int(hi-lo) + 1
			}
		}
	}
	return overlap >= 16
}

// MergeRanges 合并多份字体区间为一份（用于导出冻结快照）。
func MergeRanges(sets ...[]model.Range) []model.Range {
	var all []model.Range
	for _, rs := range sets {
		all = append(all, rs...)
	}
	return NormalizeRanges(all)
}
