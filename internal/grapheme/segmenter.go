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
		// 数字序列归组：仅同一数字系统内的连续数字合并为一个数字序列簇。
		// 不同数字系统（ASCII / Arabic-Indic / Extended Arabic-Indic /
		// Devanagari / Bengali / Thai …）在此切分为独立簇，避免被字体证明
		// 当作同一套脚本处理。
		if kind == KindSimple {
			if blk, ok := ndBlock(cp); ok {
				for i+1 < len(runes) {
					next, ok2 := ndBlock(runes[i+1])
					if !ok2 || next != blk {
						break // 跨数字系统：结束当前簇
					}
					i++
				}
				if i > start {
					kind = KindNumeric
				}
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

// ndBlocks 枚举 Unicode Nd（十进制数字）块。每个连续块对应一个独立的数字系统
// （ASCII、Arabic-Indic、Extended Arabic-Indic、Devanagari、Bengali、Thai …）。
// 不同块的数字不得并入同一数字簇，否则字体证明会把不同数字系统当作同一套脚本处理。
// 区间按升序排列，供 ndBlock 二分查找。
var ndBlocks = [...][2]rune{
	{0x0030, 0x0039}, // ASCII 数字
	{0x0660, 0x0669}, // Arabic-Indic 数字
	{0x06F0, 0x06F9}, // Extended Arabic-Indic（Persian）数字
	{0x07C0, 0x07C9}, // NKo
	{0x0966, 0x096F}, // Devanagari
	{0x09E6, 0x09EF}, // Bengali
	{0x0A66, 0x0A6F}, // Gurmukhi
	{0x0AE6, 0x0AEF}, // Gujarati
	{0x0B66, 0x0B6F}, // Oriya
	{0x0BE6, 0x0BEF}, // Tamil
	{0x0C66, 0x0C6F}, // Telugu
	{0x0CE6, 0x0CEF}, // Kannada
	{0x0D66, 0x0D6F}, // Malayalam
	{0x0DE6, 0x0DEF}, // Sinhala
	{0x0E50, 0x0E59}, // Thai
	{0x0ED0, 0x0ED9}, // Lao
	{0x0F20, 0x0F29}, // Tibetan
	{0x1040, 0x1049}, // Myanmar
	{0x1090, 0x1099}, // Myanmar Shan
	{0x17E0, 0x17E9}, // Khmer
	{0x1810, 0x1819}, // Mongolian
	{0x1946, 0x194F}, // Limbu
	{0x19D0, 0x19D9}, // New Tai Lue
	{0x1A80, 0x1A89}, // Tai Tham Hora
	{0x1A90, 0x1A99}, // Tai Tham Tham
	{0x1B50, 0x1B59}, // Balinese
	{0x1BB0, 0x1BB9}, // Sundanese
	{0x1C40, 0x1C49}, // Lepcha
	{0x1C50, 0x1C59}, // Ol Chiki
	{0xA620, 0xA629}, // Vai
	{0xA8D0, 0xA8D9}, // Saurashtra
	{0xA900, 0xA909}, // Kayah Li
	{0xA9D0, 0xA9D9}, // Javanese
	{0xA9F0, 0xA9F9}, // Myanmar Tai Laing
	{0xAA50, 0xAA59}, // Cham
	{0xABF0, 0xABF9}, // Meetei Mayek
	{0xFF10, 0xFF19}, // Fullwidth ASCII
	{0x104A0, 0x104A9}, // Osmanya
	{0x10D30, 0x10D39}, // Hanifi Rohingya
	{0x11066, 0x1106F}, // Brahmi
	{0x110F0, 0x110F9}, // Sora Sompeng
	{0x11136, 0x1113F}, // Chakma
	{0x111D0, 0x111D9}, // Sharada
	{0x112F0, 0x112F9}, // Khudawadi
	{0x11450, 0x11459}, // Newa
	{0x114D0, 0x114D9}, // Tirhuta
	{0x11650, 0x11659}, // Modi
	{0x116C0, 0x116C9}, // Takri
	{0x11730, 0x11739}, // Ahom
	{0x118E0, 0x118E9}, // Warang Citi
	{0x11950, 0x11959}, // Dives Akuru
	{0x11C50, 0x11C59}, // Bhaiksuki
	{0x11D50, 0x11D59}, // Masaram Gondi
	{0x11DA0, 0x11DA9}, // Gunjala Gondi
	{0x11F50, 0x11F59}, // Khojki
	{0x16A60, 0x16A69}, // Mro
	{0x16AC0, 0x16AC9}, // Tangsa
	{0x16B50, 0x16B59}, // Pahawh Hmong
	{0x1D7CE, 0x1D7FF}, // Mathematical digits（bold/double-struck/sans-serif…）
	{0x1E140, 0x1E149}, // Nyiakeng Puachue Hmong
	{0x1E2F0, 0x1E2F9}, // Wancho
	{0x1E4F0, 0x1E4F9}, // Nag Mundari
	{0x1E950, 0x1E959}, // Adlam
	{0x1FBF0, 0x1FBF9}, // Segmented digits
}

// ndBlock 返回十进制数字 r 所属数字系统块的块首字符，及其是否命中。
// 非数字或未登记的数字返回 (0, false)，调用方据此不并入当前数字簇。
func ndBlock(r rune) (rune, bool) {
	lo, hi := 0, len(ndBlocks)
	for lo < hi {
		mid := lo + (hi-lo)/2
		b := &ndBlocks[mid]
		if r < b[0] {
			hi = mid
		} else if r > b[1] {
			lo = mid + 1
		} else {
			return b[0], true
		}
	}
	return 0, false
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
