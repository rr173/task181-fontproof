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

func TestSegmenterNumericSystemsSeparate(t *testing.T) {
	// 不同数字系统不得拼入同一数字簇，否则字体证明会把它们当成同一套脚本。
	cases := []struct {
		name string
		text string
		want int // 期望簇数
	}{
		// ASCII 数字 + Arabic-Indic 数字 → 两个独立簇
		{"ascii+arabic-indic", "12٣٤", 2},
		// Arabic-Indic + Extended Arabic-Indic（同为 Arab 脚本，但不同数字系统）→ 拆开
		{"arabic-indic+extended", "٠١۰۱", 2},
		// ASCII + Devanagari + Bengali 数字 → 三个独立簇
		{"ascii+deva+beng", "1०০", 3},
	}
	for _, c := range cases {
		clusters := Segmenter(c.text)
		if len(clusters) != c.want {
			t.Fatalf("%s: expected %d clusters, got %d (%v)", c.name, c.want, len(clusters), clusters)
		}
		// 归组后超过 1 字符的簇应为 KindNumeric
		for i, cl := range clusters {
			if len(cl.Codepoints) > 1 && cl.Kind != KindNumeric {
				t.Fatalf("%s cluster %d: expected KindNumeric, got %v", c.name, i, cl.Kind)
			}
		}
	}

	// ASCII + Arabic-Indic 混合：第一簇应是 ASCII，第二簇应是 Arabic-Indic
	clusters := Segmenter("12٣٤")
	if clusters[0].Text != "12" {
		t.Fatalf("expected first cluster \"12\", got %q", clusters[0].Text)
	}
	if clusters[0].Kind != KindNumeric {
		t.Fatalf("expected first cluster KindNumeric, got %v", clusters[0].Kind)
	}
	if clusters[1].Text != "٣٤" {
		t.Fatalf("expected second cluster Arabic-Indic, got %q", clusters[1].Text)
	}
	if clusters[1].Kind != KindNumeric {
		t.Fatalf("expected second cluster KindNumeric, got %v", clusters[1].Kind)
	}
	if clusters[1].Script != "Arab" {
		t.Fatalf("expected Arabic-Indic cluster script Arab, got %q", clusters[1].Script)
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
