package proof

import (
	"testing"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/typeface"
)

func buildEngine(rangesByFont map[string][]model.Range, order []string, fontsByRule map[string][]string) *Engine {
	var fonts []model.Font
	var ranges []model.FontRange
	for id, rs := range rangesByFont {
		fonts = append(fonts, model.Font{ID: id, Name: id})
		for _, r := range rs {
			ranges = append(ranges, model.FontRange{FontID: id, Start: r.Start, End: r.End})
		}
	}
	view := typeface.NewCoverageView(fonts, ranges)
	return &Engine{View: view, Order: order, FontsByRule: fontsByRule}
}

func TestProveClusterCovered(t *testing.T) {
	e := buildEngine(map[string][]model.Range{
		"latin": {{Start: 0x0041, End: 0x007A}},
	}, []string{"r1"}, map[string][]string{"r1": {"latin"}})
	res := e.ProveCluster([]rune{'c', 'a', 't'}, false, 0)
	if res.Status != model.ClusterCovered {
		t.Fatalf("expected covered, got %s", res.Status)
	}
	if len(res.Chain) != 1 || res.Chain[0] != "latin" {
		t.Fatalf("unexpected chain: %v", res.Chain)
	}
}

func TestProveClusterCompositeRisk(t *testing.T) {
	// a + U+0301：latin 覆盖 a，mark 覆盖 U+0301 → 组合簇被拆 → risk
	e := buildEngine(map[string][]model.Range{
		"latin": {{Start: 0x0041, End: 0x007A}},
		"mark":  {{Start: 0x0300, End: 0x036F}},
	}, []string{"r1"}, map[string][]string{"r1": {"latin", "mark"}})
	res := e.ProveCluster([]rune{'a', 0x0301}, true, 2)
	if res.Status != model.ClusterRisk {
		t.Fatalf("expected risk, got %s", res.Status)
	}
	if len(res.Chain) != 2 {
		t.Fatalf("expected 2-font chain, got %v", res.Chain)
	}
}

func TestProveClusterNumericSplitRisk(t *testing.T) {
	// 数字序列 12345：latin 覆盖 1-5 前半，digits 覆盖其余 → 数字序列被拆 → risk
	e := buildEngine(map[string][]model.Range{
		"latin":  {{Start: 0x0030, End: 0x0032}}, // 0-2
		"digits": {{Start: 0x0033, End: 0x0039}}, // 3-9
	}, []string{"r1"}, map[string][]string{"r1": {"latin", "digits"}})
	res := e.ProveCluster([]rune{'1', '2', '3'}, true, 6)
	if res.Status != model.ClusterRisk {
		t.Fatalf("expected risk for split numeric sequence, got %s", res.Status)
	}
}

func TestProveClusterMissing(t *testing.T) {
	// क + ा（U+0915, U+093E）无任何字体覆盖 → missing
	e := buildEngine(map[string][]model.Range{
		"latin": {{Start: 0x0041, End: 0x007A}},
	}, []string{"r1"}, map[string][]string{"r1": {"latin"}})
	res := e.ProveCluster([]rune{0x0915, 0x093E}, true, 2)
	if res.Status != model.ClusterMissing {
		t.Fatalf("expected missing, got %s", res.Status)
	}
	if len(res.Missing) != 2 {
		t.Fatalf("expected 2 missing codepoints, got %v", res.Missing)
	}
}

func TestAnalyzeStats(t *testing.T) {
	// 样本：a（covered）、a+U+0301（risk）、क（missing）
	e := buildEngine(map[string][]model.Range{
		"latin": {{Start: 0x0041, End: 0x007A}},
		"mark":  {{Start: 0x0300, End: 0x036F}},
	}, []string{"r1"}, map[string][]string{"r1": {"latin", "mark"}})
	gs, stats, err := Analyze(e, "aa\u0301\u0915")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 3 {
		t.Fatalf("expected 3 total, got %d", stats.Total)
	}
	if stats.Covered != 1 || stats.Risk != 1 || stats.Missing != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(gs) != 3 {
		t.Fatalf("expected 3 graphemes, got %d", len(gs))
	}
}

func TestPassed(t *testing.T) {
	if Passed(model.AnalysisStats{Missing: 0, Risk: 0}) != true {
		t.Fatal("no gaps should pass")
	}
	if Passed(model.AnalysisStats{Missing: 1}) != false {
		t.Fatal("missing should not pass")
	}
}
