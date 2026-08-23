// Package grapheme 实现字素簇切分（UAX#29 简化子集）与脚本检测。
//
// 覆盖以下扩展字素簇规则（GB9/GB9a/GB9b 的核心部分）：
//   - 基础字符 + 组合标记（Mn/Me/Mc）合并；
//   - 变体选择符（U+FE00..FE0F、U+E0100..E01EF）附加到前一个字符；
//   - ZWJ（U+200D）连接前后字符；
//   - Emoji 修饰符（U+1F3FB..1F3FF）附加；
//   - 同一数字系统内的连续数字归组为数字序列簇（防止数字系统被错误拆分）。
package grapheme

import (
	"unicode"
	"unicode/utf8"
)

// Cluster 是切分出的一个字素簇。
type Cluster struct {
	Text        string
	Codepoints  []rune
	Script      string
	IsComposite bool // 是否复合簇（含组合标记/变体/ZWJ/修饰符/数字序列）
	Kind        ClusterKind
}

// ClusterKind 标识簇的类型，用于拆分风险评估。
type ClusterKind int

const (
	KindSimple    ClusterKind = iota // 简单单字符簇
	KindCombining                    // 含组合标记的复合簇
	KindVariation                    // 含变体选择符
	KindZWJ                          // 含 ZWJ 连接序列
	KindModifier                     // 含 Emoji 修饰符
	KindNumeric                      // 数字序列簇
)

// Segmenter 将输入文本切分为字素簇序列。
func Segmenter(text string) []Cluster {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	var clusters []Cluster
	i := 0
	for i < len(runes) {
		start := i
		cp := runes[i]
		kind := KindSimple

		// 附加变体选择符（U+FE0F 等同时属于 Mn，必须先于组合标记分支处理）
		for i+1 < len(runes) && isVariationSelector(runes[i+1]) {
			i++
			kind = KindVariation
		}
		// 附加组合标记
		for i+1 < len(runes) && isCombiningMark(runes[i+1]) {
			i++
			if kind == KindSimple {
				kind = KindCombining
			}
		}
		// ZWJ 序列：当前字符后 ZWJ + 下一基础字符（可递归附加后续组合）
		for i+1 < len(runes) && runes[i+1] == '\u200d' {
			i += 2 // 跳过 ZWJ 与下一个字符
			kind = KindZWJ
			for i+1 < len(runes) && isCombiningMark(runes[i+1]) {
				i++
			}
		}
		// Emoji 修饰符
		if i+1 < len(runes) && isEmojiModifier(runes[i+1]) {
			i++
			kind = KindModifier
		}
		// 数字序列归组：仅当以数字开头且尚未并入组合内容时
		if kind == KindSimple && isDigit(cp) {
			for i+1 < len(runes) && isDigit(runes[i+1]) {
				i++
			}
			if i > start {
				kind = KindNumeric
			}
		}
		// 若簇内字符数大于 1 且不是数字序列，标记复合
		if i > start && kind == KindSimple {
			kind = KindCombining
		}
		seg := runes[start : i+1]
		clusters = append(clusters, Cluster{
			Text:        string(seg),
			Codepoints:  append([]rune(nil), seg...),
			Script:      DetectScript(seg),
			IsComposite: i > start,
			Kind:        kind,
		})
		i++
	}
	return clusters
}

// isCombiningMark 判断是否为组合标记（Mn/Me/Mc）。
func isCombiningMark(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Mc, r)
}

// isVariationSelector 判断是否为变体选择符。
func isVariationSelector(r rune) bool {
	return (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0xE0100 && r <= 0xE01EF)
}

// isEmojiModifier 判断是否为 Emoji 肤色修饰符。
func isEmojiModifier(r rune) bool {
	return r >= 0x1F3FB && r <= 0x1F3FF
}

// isDigit 判断是否为十进制数字（任意脚本的数字系统）。
func isDigit(r rune) bool {
	return unicode.IsNumber(r)
}

// RuneCount 返回文本的码点数量。
func RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}

// Codepoints 返回文本的码点切片。
func Codepoints(s string) []rune {
	return []rune(s)
}

// FormatCodepoints 将码点格式化为 U+XXXX 列表，用于缺失证明展示。
func FormatCodepoints(cps []rune) string {
	out := ""
	for i, cp := range cps {
		if i > 0 {
			out += " "
		}
		out += runeHex(cp)
	}
	return out
}

func runeHex(cp rune) string {
	const hexDigits = "0123456789ABCDEF"
	// 最小 4 位，必要时扩展到 5/6 位
	width := 4
	for cp>>(uint(width)*4) > 0 {
		width++
	}
	buf := make([]byte, width)
	v := cp
	for i := width - 1; i >= 0; i-- {
		buf[i] = hexDigits[v&0xF]
		v >>= 4
	}
	return "U+" + string(buf)
}
