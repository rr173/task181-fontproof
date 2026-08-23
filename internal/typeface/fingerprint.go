// Package typeface 负责字体资产的覆盖摘要管理：指纹、Unicode 范围解析与覆盖查询。
package typeface

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"task181-fontproof/internal/model"
)

// Fingerprint 基于字体名称、族、规范版本与归一化后的覆盖范围计算稳定指纹。
// 相同指纹的字体摘要重复导入时复用扫描结果（幂等登记）。
func Fingerprint(in model.FontInput) string {
	features := append([]string(nil), in.Features...)
	scripts := append([]string(nil), in.Scripts...)
	h := sha256.New()
	h.Write([]byte(strings.ToLower(strings.TrimSpace(in.Name))))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToLower(strings.TrimSpace(in.Family))))
	h.Write([]byte{0})
	h.Write([]byte(strings.TrimSpace(in.SpecVersion)))
	h.Write([]byte{0})
	for _, r := range NormalizeRanges(in.Ranges) {
		fmt.Fprintf(h, "%04x-%04x;", r.Start, r.End)
	}
	sort.Strings(features)
	for _, f := range features {
		fmt.Fprintf(h, "f:%s;", f)
	}
	sort.Strings(scripts)
	for _, s := range scripts {
		fmt.Fprintf(h, "s:%s;", s)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// NormalizeRanges 合并相邻/重叠区间并排序，返回不可变归一化区间列表。
func NormalizeRanges(rs []model.Range) []model.Range {
	if len(rs) == 0 {
		return nil
	}
	sorted := make([]model.Range, len(rs))
	copy(sorted, rs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Start == sorted[j].Start {
			return sorted[i].End < sorted[j].End
		}
		return sorted[i].Start < sorted[j].Start
	})
	var out []model.Range
	for _, r := range sorted {
		if r.End < r.Start {
			continue
		}
		if len(out) == 0 {
			out = append(out, r)
			continue
		}
		last := &out[len(out)-1]
		if r.Start <= last.End+1 {
			if r.End > last.End {
				last.End = r.End
			}
			continue
		}
		out = append(out, r)
	}
	return out
}

// CountCodepoints 计算归一化区间覆盖的码点总数。
func CountCodepoints(rs []model.Range) int {
	total := 0
	for _, r := range NormalizeRanges(rs) {
		total += int(r.End-r.Start) + 1
	}
	return total
}

// Contains 判断归一化区间是否覆盖给定码点。
func Contains(rs []model.Range, cp rune) bool {
	for _, r := range rs {
		if cp < r.Start {
			return false
		}
		if cp <= r.End {
			return true
		}
	}
	return false
}

// MissingCodepoints 返回区间集合中未覆盖的码点（保持输入顺序去重）。
func MissingCodepoints(rs []model.Range, cps []rune) []rune {
	var missing []rune
	seen := make(map[rune]bool)
	for _, cp := range cps {
		if seen[cp] {
			continue
		}
		seen[cp] = true
		if !Contains(rs, cp) {
			missing = append(missing, cp)
		}
	}
	return missing
}

// DescribeRanges 生成人类可读的区间描述，如 "0000-007F; 0600-06FF"。
func DescribeRanges(rs []model.Range) string {
	parts := make([]string, 0, len(rs))
	for _, r := range NormalizeRanges(rs) {
		parts = append(parts, fmt.Sprintf("%04X-%04X", r.Start, r.End))
	}
	return strings.Join(parts, "; ")
}
