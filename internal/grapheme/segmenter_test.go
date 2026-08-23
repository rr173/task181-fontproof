package grapheme

import "testing"

func TestSegmenterCombiningMarks(t *testing.T) {
	// "a\u0301"：a + 组合重音（U+0301）应合并为一个簇
	clusters := Segmenter("a\u0301")
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster for combining mark, got %d", len(clusters))
	}
	if !clusters[0].IsComposite {
		t.Fatal("expected composite cluster")
	}
	if clusters[0].Kind != KindCombining {
		t.Fatalf("expected KindCombining, got %v", clusters[0].Kind)
	}
	if clusters[0].Text != "a\u0301" {
		t.Fatalf("unexpected cluster text %q", clusters[0].Text)
	}
}

func TestSegmenterVariationSelector(t *testing.T) {
	// 基础字符 + 变体选择符（U+FE0F）合并
	clusters := Segmenter("\u2764\uFE0F") // ❤ + VS16
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster with variation selector, got %d", len(clusters))
	}
	if clusters[0].Kind != KindVariation {
		t.Fatalf("expected KindVariation, got %v", clusters[0].Kind)
	}
}

func TestSegmenterZWJ(t *testing.T) {
	// 👩 + ZWJ + 💻 应合并为一个簇
	clusters := Segmenter("\U0001F469\u200D\U0001F4BB")
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster for ZWJ sequence, got %d", len(clusters))
	}
	if clusters[0].Kind != KindZWJ {
		t.Fatalf("expected KindZWJ, got %v", clusters[0].Kind)
	}
}

func TestSegmenterNumericGrouping(t *testing.T) {
	// 连续数字归组为一个数字序列簇（防止数字系统错误拆分）
	clusters := Segmenter("12345")
	if len(clusters) != 1 {
		t.Fatalf("expected 1 numeric cluster, got %d", len(clusters))
	}
	if clusters[0].Kind != KindNumeric {
		t.Fatalf("expected KindNumeric, got %v", clusters[0].Kind)
	}
	if len(clusters[0].Codepoints) != 5 {
		t.Fatalf("expected 5 codepoints, got %d", len(clusters[0].Codepoints))
	}
}

func TestSegmenterMixed(t *testing.T) {
	clusters := Segmenter("héllo")
	if len(clusters) != 5 {
		t.Fatalf("expected 5 clusters, got %d", len(clusters))
	}
}

func TestDetectScript(t *testing.T) {
	cases := []struct {
		seq    []rune
		expect string
	}{
		{[]rune{'a'}, "Latn"},
		{[]rune{0x0627}, "Arab"}, // ا
		{[]rune{0x0915}, "Deva"}, // क
		{[]rune{'1'}, "Zyyy"},
		// 带组合重音的拉丁字素簇：组合标记应沿用基础字符的脚本，
		// 而非被标成继承脚本（Zinh）
		{[]rune{'a', 0x0301}, "Latn"},
		// 基础字符 + 变体选择符同样沿用基础脚本
		{[]rune{'a', 0xFE0F}, "Latn"},
	}
	for _, c := range cases {
		if got := DetectScript(c.seq); got != c.expect {
			t.Errorf("DetectScript(%v) = %s, want %s", c.seq, got, c.expect)
		}
	}
}

func TestFormatCodepoints(t *testing.T) {
	got := FormatCodepoints([]rune{0x0915, 0x093E})
	want := "U+0915 U+093E"
	if got != want {
		t.Fatalf("FormatCodepoints = %q, want %q", got, want)
	}
}
