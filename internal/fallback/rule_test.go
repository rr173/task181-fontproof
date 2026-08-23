package fallback

import (
	"strings"
	"testing"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/typeface"
)

// stubView 构建测试用覆盖视图。
func stubView(rangesByFont map[string][]model.Range) *typeface.CoverageView {
	var fonts []model.Font
	var ranges []model.FontRange
	for id, rs := range rangesByFont {
		fonts = append(fonts, model.Font{ID: id, Name: id})
		for _, r := range rs {
			ranges = append(ranges, model.FontRange{FontID: id, Start: r.Start, End: r.End})
		}
	}
	return typeface.NewCoverageView(fonts, ranges)
}

func TestChecksumStable(t *testing.T) {
	rules := []model.FallbackRule{
		{ID: "r1", Name: "a", Priority: 1, RequiredScripts: []string{"Latn"}},
		{ID: "r2", Name: "b", Priority: 2, RequiredScripts: []string{"Arab"}},
	}
	fonts := map[string][]string{
		"r1": {"f1", "f2"},
		"r2": {"f3"},
	}
	c1 := Checksum(rules, fonts)
	c2 := Checksum(rules, fonts)
	if c1 != c2 {
		t.Fatal("checksum should be deterministic")
	}
}

func TestChecksumChangesOnReorder(t *testing.T) {
	rules := []model.FallbackRule{
		{ID: "r1", Name: "a", Priority: 1},
		{ID: "r2", Name: "b", Priority: 2},
	}
	fonts := map[string][]string{"r1": {"f1"}, "r2": {"f2"}}
	before := Checksum(rules, fonts)
	res, err := ApplyReorder(rules, fonts, []string{"r2", "r1"}, before)
	if err != nil {
		t.Fatalf("reorder failed: %v", err)
	}
	if res.Checksum == before {
		t.Fatal("checksum should change after reorder")
	}
	if res.Rules[0].ID != "r2" || res.Rules[0].Priority != 1 {
		t.Fatalf("reorder not applied: %+v", res.Rules[0])
	}
}

func TestApplyReorderVersionConflict(t *testing.T) {
	rules := []model.FallbackRule{
		{ID: "r1", Name: "a", Priority: 1},
		{ID: "r2", Name: "b", Priority: 2},
	}
	fonts := map[string][]string{"r1": {"f1"}, "r2": {"f2"}}
	// 另一工程师已重排（checksum 变化），调用方仍持旧 checksum → 冲突
	current := Checksum(rules, fonts)
	_, err := ApplyReorder(rules, fonts, []string{"r1", "r2"}, "stale-checksum")
	if err == nil {
		t.Fatal("expected checksum mismatch conflict")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = current
}

func TestCheckMutualExclusion(t *testing.T) {
	a := model.FallbackRule{ID: "r1", Priority: 3, RequiredScripts: []string{"Latn"}}
	b := model.FallbackRule{ID: "r2", Priority: 3, RequiredScripts: []string{"Latn"}}
	if err := CheckMutualExclusion(a, b, []string{"f1"}, []string{"f1"}); err == nil {
		t.Fatal("expected mutual exclusion error for identical priority+fonts")
	}
	c := model.FallbackRule{ID: "r3", Priority: 4}
	if err := CheckMutualExclusion(a, c, []string{"f1"}, []string{"f1"}); err != nil {
		t.Fatalf("different priority should not be exclusive: %v", err)
	}
}

func TestCheckCycle(t *testing.T) {
	// f1 → f2 → f3 无环
	if err := CheckCycle(map[string][]string{"r1": {"f1", "f2", "f3"}}); err != nil {
		t.Fatalf("expected no cycle: %v", err)
	}
	// f1 → f2 且 f2 → f1（跨规则）成环
	cyc := map[string][]string{
		"r1": {"f1", "f2"},
		"r2": {"f2", "f1"},
	}
	if err := CheckCycle(cyc); err == nil {
		t.Fatal("expected cycle detected")
	}
}

func TestValidateRuleRequiredScript(t *testing.T) {
	// 必需脚本无字体覆盖 → gapped
	snap := RuleSnapshot{
		Rule:    model.FallbackRule{RequiredScripts: []string{"Deva"}},
		FontIDs: []string{"f1"},
	}
	view := stubView(map[string][]model.Range{
		"f1": {{Start: 0x0041, End: 0x007A}},
	})
	vr := ValidateRule(snap, view)
	if vr.Status != model.RuleGapped {
		t.Fatalf("expected gapped, got %s", vr.Status)
	}
}

func TestResolveOrder(t *testing.T) {
	rules := []model.FallbackRule{
		{ID: "r2", Priority: 2},
		{ID: "r1", Priority: 1},
	}
	order := ResolveOrder(rules)
	if order[0] != "r1" || order[1] != "r2" {
		t.Fatalf("unexpected order: %v", order)
	}
}
