package typeface

import (
	"sort"

	"task181-fontproof/internal/model"
)

// CoverageView 是一次分析所需的字体覆盖视图：按脚本索引可查询的字体集合。
type CoverageView struct {
	// RangesByFont 字体 id → 归一化区间
	RangesByFont map[string][]model.Range
	// FontNames 字体 id → 名称
	FontNames map[string]string
	// ScriptsByFont 字体 id → 覆盖脚本集合
	ScriptsByFont map[string][]string
}

// NewCoverageView 从字体与区间构建覆盖视图。
func NewCoverageView(fonts []model.Font, ranges []model.FontRange) *CoverageView {
	v := &CoverageView{
		RangesByFont:  make(map[string][]model.Range),
		FontNames:     make(map[string]string),
		ScriptsByFont: make(map[string][]string),
	}
	for _, f := range fonts {
		v.FontNames[f.ID] = f.Name
	}
	for _, r := range ranges {
		v.RangesByFont[r.FontID] = append(v.RangesByFont[r.FontID], model.Range{Start: r.Start, End: r.End})
	}
	for id, rs := range v.RangesByFont {
		v.RangesByFont[id] = NormalizeRanges(rs)
	}
	return v
}

// Covers 判断字体是否覆盖给定码点。
func (v *CoverageView) Covers(fontID string, cp rune) bool {
	return Contains(v.RangesByFont[fontID], cp)
}

// CoversAll 判断字体是否覆盖所有给定码点。
func (v *CoverageView) CoversAll(fontID string, cps []rune) bool {
	for _, cp := range cps {
		if !v.Covers(fontID, cp) {
			return false
		}
	}
	return true
}

// CoversAnyScript 判断字体覆盖集合是否触及指定脚本（任一基础区间有交集）。
func (v *CoverageView) CoversAnyScript(fontID, script string) bool {
	return coversScript(v.RangesByFont[fontID], script)
}

// ScriptFonts 返回覆盖指定脚本的字体 id（按名称稳定排序，保证确定性输出）。
func (v *CoverageView) ScriptFonts(script string) []string {
	var out []string
	for id, rs := range v.RangesByFont {
		if coversScript(rs, script) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return v.FontNames[out[i]] < v.FontNames[out[j]] })
	return out
}

// coversScript 通过脚本的基础区间表判断字体覆盖集合是否触及该脚本。
func coversScript(rs []model.Range, script string) bool {
	ranges, ok := scriptBaseRanges[script]
	if !ok {
		return false
	}
	for _, sr := range ranges {
		for _, r := range rs {
			if r.Start <= sr.End && sr.Start <= r.End {
				return true
			}
		}
	}
	return false
}

// scriptBaseRanges 脚本 → 代表性基础区间（用于必需脚本覆盖判定）。
var scriptBaseRanges = map[string][]model.Range{
	"Latn": {{Start: 0x0041, End: 0x007A}, {Start: 0x00C0, End: 0x024F}},
	"Arab": {{Start: 0x0600, End: 0x06FF}, {Start: 0x0750, End: 0x077F}},
	"Deva": {{Start: 0x0900, End: 0x097F}},
	"Beng": {{Start: 0x0980, End: 0x09FF}},
	"Hani": {{Start: 0x4E00, End: 0x9FFF}},
	"Hira": {{Start: 0x3040, End: 0x309F}},
	"Kana": {{Start: 0x30A0, End: 0x30FF}},
	"Hang": {{Start: 0xAC00, End: 0xD7AF}},
	"Grek": {{Start: 0x0370, End: 0x03FF}},
	"Cyrl": {{Start: 0x0400, End: 0x04FF}},
	"Thai": {{Start: 0x0E00, End: 0x0E7F}},
	"Zyyy": {{Start: 0x0000, End: 0x007F}, {Start: 0x2000, End: 0x206F}},
}
