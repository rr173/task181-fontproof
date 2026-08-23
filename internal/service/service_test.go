package service

import (
	"path/filepath"
	"testing"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/store"
)

// newTestService 构造内存数据库服务。
func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st)
}

// seedFontsRules 登记一组字体与规则（返回字体 id）。
func seedFontsRules(t *testing.T, svc *Service) []string {
	t.Helper()
	fonts := []model.FontInput{
		{Name: "Noto Sans Latin", Family: "Noto Sans", Ranges: []model.Range{{Start: 0x0020, End: 0x007E}, {Start: 0x00C0, End: 0x024F}}, Scripts: []string{"Latn", "Zyyy"}},
		{Name: "Noto Sans Mark", Family: "Noto Sans", Ranges: []model.Range{{Start: 0x0300, End: 0x036F}}, Scripts: []string{"Zinh"}},
		{Name: "Noto Naskh Arabic", Family: "Noto Naskh", Ranges: []model.Range{{Start: 0x0600, End: 0x06FF}}, Scripts: []string{"Arab"}},
	}
	var ids []string
	for _, in := range fonts {
		res, err := svc.RegisterFont(in)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, res.Font.ID)
		if _, err := svc.ScanFont(res.Font.ID); err != nil {
			t.Fatal(err)
		}
	}
	r1, err := svc.CreateRule(model.RuleInput{Name: "latin-fallback", RequiredScripts: []string{"Latn"}, FontIDs: []string{ids[0], ids[1]}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublishRule(r1.Rule.ID); err != nil {
		t.Fatal(err)
	}
	r2, err := svc.CreateRule(model.RuleInput{Name: "arabic-fallback", RequiredScripts: []string{"Arab"}, FontIDs: []string{ids[2]}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublishRule(r2.Rule.ID); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestFontRegisterIdempotent(t *testing.T) {
	svc := newTestService(t)
	in := model.FontInput{Name: "A", Family: "F", Ranges: []model.Range{{Start: 0x0041, End: 0x005A}}, Scripts: []string{"Latn"}}
	r1, err := svc.RegisterFont(in)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.RegisterFont(in)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Font.ID != r2.Font.ID {
		t.Fatalf("fingerprint idempotency broken: %s != %s", r1.Font.ID, r2.Font.ID)
	}
	if !r2.Reused {
		t.Fatal("expected reuse flag")
	}
}

func TestFontStateMachine(t *testing.T) {
	svc := newTestService(t)
	res, err := svc.RegisterFont(model.FontInput{Name: "A", Family: "F", Ranges: []model.Range{{Start: 0x0041, End: 0x005A}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Font.Status != model.FontPendingScan {
		t.Fatalf("expected pending_scan, got %s", res.Font.Status)
	}
	f, err := svc.ScanFont(res.Font.ID)
	if err != nil {
		t.Fatal(err)
	}
	if f.Status != model.FontAvailable {
		t.Fatalf("expected available, got %s", f.Status)
	}
	if _, err := svc.DisableFont(res.Font.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnableFont(res.Font.ID); err != nil {
		t.Fatal(err)
	}
}

func TestPublishedRuleCannotEdit(t *testing.T) {
	svc := newTestService(t)
	ids := seedFontsRules(t, svc)
	rules, _ := svc.ListRules()
	published := rules[0]
	_, err := svc.UpdateRule(published.Rule.ID, model.RuleInput{Name: "hacked", FontIDs: []string{ids[0]}}, published.Rule.Version)
	if err == nil {
		t.Fatal("expected forbidden error editing published rule")
	}
}

func TestReorderChecksumConflict(t *testing.T) {
	svc := newTestService(t)
	seedFontsRules(t, svc)
	state, err := svc.RuleSetChecksum()
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := svc.ListRules()
	newOrder := []string{rules[1].Rule.ID, rules[0].Rule.ID}
	// 正确的 checksum 应成功
	if _, err := svc.ReorderRules(newOrder, state.Checksum); err != nil {
		t.Fatalf("reorder with fresh checksum should succeed: %v", err)
	}
	// 过期的 checksum 应冲突
	if _, err := svc.ReorderRules([]string{rules[0].Rule.ID, rules[1].Rule.ID}, "stale-checksum"); err == nil {
		t.Fatal("expected checksum conflict")
	}
}

func TestFullPipelineAndVersionFreeze(t *testing.T) {
	svc := newTestService(t)
	seedFontsRules(t, svc)
	// 样本含组合字符（风险）与 Devanagari（缺字）
	sample, err := svc.CreateSampleSet("s1", "a\u0301 \u0915")
	if err != nil {
		t.Fatal(err)
	}
	an, err := svc.StartAnalysis(sample.SampleSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if an.Status != model.AnalysisCompleted {
		t.Fatalf("expected completed analysis, got %s", an.Status)
	}
	risks, _ := svc.AnalysisRisks(an.ID)
	gaps, _ := svc.AnalysisGaps(an.ID)
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d", len(risks))
	}
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}
	// 报告有缺口
	rep, err := svc.GenerateReport(an.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Stats.Missing != 1 {
		t.Fatalf("expected 1 missing in report, got %d", rep.Stats.Missing)
	}
	// 发布配置
	cfg1, err := svc.PublishConfig("v1")
	if err != nil {
		t.Fatal(err)
	}
	// 替换字体覆盖摘要：新增 Devanagari 字体与新规则 → 新报告无缺口，旧配置不变
	deva, err := svc.RegisterFont(model.FontInput{Name: "Noto Sans Devanagari", Family: "Noto Sans", Ranges: []model.Range{{Start: 0x0900, End: 0x097F}}, Scripts: []string{"Deva"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ScanFont(deva.Font.ID); err != nil {
		t.Fatal(err)
	}
	r3, err := svc.CreateRule(model.RuleInput{Name: "deva-fallback", RequiredScripts: []string{"Deva"}, FontIDs: []string{deva.Font.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublishRule(r3.Rule.ID); err != nil {
		t.Fatal(err)
	}
	sample2, err := svc.CreateSampleSet("s2", "\u0915")
	if err != nil {
		t.Fatal(err)
	}
	an2, err := svc.StartAnalysis(sample2.SampleSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	rep2, err := svc.GenerateReport(an2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Stats.Missing != 0 || rep2.Stats.Risk != 0 {
		t.Fatalf("v2 report should pass: %+v", rep2.Stats)
	}
	cfg2, err := svc.PublishConfig("v2")
	if err != nil {
		t.Fatal(err)
	}
	oldCfg, err := svc.GetPublishedConfig(cfg1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if oldCfg.Status != model.ConfigSuperseded {
		t.Fatalf("expected v1 superseded, got %s", oldCfg.Status)
	}
	if oldCfg.Snapshot != cfg1.Snapshot {
		t.Fatal("old config snapshot changed")
	}
	diff, err := svc.CompareConfigs(cfg1.ID, cfg2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Same {
		t.Fatal("configs should differ")
	}
}

func TestRecoverInterruptedAnalysis(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(st)
	seedFontsRules(t, svc)
	sample, err := svc.CreateSampleSet("s", "abc")
	if err != nil {
		t.Fatal(err)
	}
	// 模拟崩溃：写入 running 分析但未完成
	an := model.Analysis{
		ID:          "anl-interrupted",
		SampleSetID: sample.SampleSet.ID,
		Status:      model.AnalysisRunning,
		RuleVersion: 1,
		Stats:       "{}",
	}
	if err := st.CreateAnalysis(an); err != nil {
		t.Fatal(err)
	}
	st.Close()

	// 重开：Recover 应标记 resumed
	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	svc2 := New(st2)
	recovered, err := svc2.Recover()
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered) != 1 || recovered[0] != an.ID {
		t.Fatalf("expected interrupted analysis recovered, got %v", recovered)
	}
	// 继续分析完成
	done, err := svc2.ResolveAnalysis(an.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != model.AnalysisCompleted {
		t.Fatalf("expected completed, got %s", done.Status)
	}
}
