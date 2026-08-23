package grapheme

import "unicode"

// DetectScript 推断一个码点序列的主要 Unicode 脚本。
// 按“第一个非公共脚本字符”优先；组合标记与变体选择符继承前一个字符的脚本。
func DetectScript(seq []rune) string {
	inherited := ""
	for _, cp := range seq {
		if s, ok := scriptOf(cp); ok && s != "Zyyy" && s != "Zinh" {
			return s
		}
	}
	// 全部是公共/继承字符：用第一个字符的归属（若有）
	for _, cp := range seq {
		if s, ok := scriptOf(cp); ok {
			return s
		}
	}
	return inherited
}

// SplitReason returns the user-facing reason for a composite cluster being
// covered by more than one font. Keep the mapping beside ClusterKind so the
// proof layer cannot drift from the segmenter's enum values.
func SplitReason(kind ClusterKind) string {
	switch kind {
	case KindCombining:
		return "combining mark split from base character"
	case KindVariation:
		return "variation selector split from base character"
	case KindZWJ:
		return "ZWJ sequence split across fonts"
	case KindModifier:
		return "emoji modifier split from base emoji"
	case KindNumeric:
		return "numeric sequence split across fonts (style inconsistency risk)"
	default:
		return "composite grapheme split across fonts"
	}
}

// scriptOf 返回单个码点的脚本标签；未识别返回 Zzzz。
func scriptOf(r rune) (string, bool) {
	if unicode.Is(unicode.S, r) && isEmojiLike(r) {
		return "Zyyy", true
	}
	switch {
	case r >= 0x0041 && r <= 0x005A, r >= 0x0061 && r <= 0x007A:
		return "Latn", true
	case r >= 0x00C0 && r <= 0x024F:
		return "Latn", true
	case r >= 0x0370 && r <= 0x03FF:
		return "Grek", true
	case r >= 0x0400 && r <= 0x04FF:
		return "Cyrl", true
	case r >= 0x0600 && r <= 0x06FF, r >= 0x0750 && r <= 0x077F, r >= 0x08A0 && r <= 0x08FF:
		return "Arab", true
	case r >= 0x0900 && r <= 0x097F:
		return "Deva", true
	case r >= 0x0980 && r <= 0x09FF:
		return "Beng", true
	case r >= 0x0E00 && r <= 0x0E7F:
		return "Thai", true
	case r >= 0x3040 && r <= 0x309F:
		return "Hira", true
	case r >= 0x30A0 && r <= 0x30FF:
		return "Kana", true
	case r >= 0x4E00 && r <= 0x9FFF, r >= 0x3400 && r <= 0x4DBF:
		return "Hani", true
	case r >= 0xAC00 && r <= 0xD7AF:
		return "Hang", true
	case r >= 0x0300 && r <= 0x036F, r >= 0x1AB0 && r <= 0x1AFF, r >= 0x1DC0 && r <= 0x1DFF:
		return "Zinh", true
	case (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0xE0100 && r <= 0xE01EF):
		return "Zinh", true
	case r >= 0x200D:
		return "Zyyy", true
	case unicode.Is(unicode.Nd, r):
		return "Zyyy", true
	case r >= 0x0000 && r <= 0x007F, r >= 0x2000 && r <= 0x206F:
		return "Zyyy", true
	}
	return "Zzzz", false
}

// isEmojiLike 粗判 emoji 区间（供脚本归属使用）。
func isEmojiLike(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF)
}

// IsCombiningOnly 判断簇是否全部由组合标记构成（异常输入防御）。
func IsCombiningOnly(cl Cluster) bool {
	for _, cp := range cl.Codepoints {
		if !isCombiningMark(cp) && !isVariationSelector(cp) {
			return false
		}
	}
	return len(cl.Codepoints) > 0
}

// ScriptBaseRange 返回脚本的代表性码点（用于构造必需脚本覆盖样本）。
func ScriptBaseRange(script string) (start, end rune, ok bool) {
	ranges := map[string][2]rune{
		"Latn": {0x0041, 0x007A}, "Arab": {0x0600, 0x06FF}, "Deva": {0x0900, 0x097F},
		"Beng": {0x0980, 0x09FF}, "Hani": {0x4E00, 0x9FFF}, "Hira": {0x3040, 0x309F},
		"Kana": {0x30A0, 0x30FF}, "Hang": {0xAC00, 0xD7AF}, "Grek": {0x0370, 0x03FF},
		"Cyrl": {0x0400, 0x04FF}, "Thai": {0x0E00, 0x0E7F}, "Zyyy": {0x0020, 0x007E},
	}
	if r, ok := ranges[script]; ok {
		return r[0], r[1], true
	}
	return 0, 0, false
}
