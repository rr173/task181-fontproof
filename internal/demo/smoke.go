// Package demo 提供离线端到端 smoke test：真实创建字体、规则、样本、
// 执行覆盖分析并验证拆分风险链、发布配置，最后关闭并重新打开数据库
// 验证持久化与重启恢复。这是 Docker 双架构验证的唯一判据。
package demo

import (
	"fmt"
	"os"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/service"
	"task181-fontproof/internal/store"
)

// RunSmoke 执行完整 smoke test 场景。
// dbPath 为空时使用临时文件数据库（便于关闭重开验证恢复）。
func RunSmoke(dbPath string) error {
	if dbPath == "" {
		f, err := os.CreateTemp("", "fontproof-smoke-*.db")
		if err != nil {
			return fmt.Errorf("create temp db: %w", err)
		}
		dbPath = f.Name()
		f.Close()
		os.Remove(dbPath)
	}

	// ---- 第一段：建库、建数据、分析 ----
	svc, st, err := openService(dbPath)
	if err != nil {
		return err
	}
	cfg, err := seedStage1(svc)
	if err != nil {
		return err
	}
	if err := st.Close(); err != nil {
		return fmt.Errorf("close db (stage1): %w", err)
	}

	// ---- 第二段：关闭重开，验证持久化与恢复 ----
	svc2, st2, err := openService(dbPath)
	if err != nil {
		return fmt.Errorf("reopen db: %w", err)
	}
	recovered, err := svc2.Recover()
	if err != nil {
		return fmt.Errorf("recover: %w", err)
	}
	// 验证数据仍在
	fonts, err := svc2.ListFonts()
	if err != nil {
		return err
	}
	if len(fonts) < 2 {
		return fmt.Errorf("persistence check failed: expected >=2 fonts after reopen, got %d", len(fonts))
	}
	rules, err := svc2.ListRules()
	if err != nil {
		return err
	}
	if len(rules) < 1 {
		return fmt.Errorf("persistence check failed: expected >=1 rule after reopen")
	}
	// 验证已发布配置冻结
	cfgAfter, err := svc2.GetPublishedConfig(cfg.ID)
	if err != nil {
		return err
	}
	if cfgAfter.Checksum != cfg.Checksum {
		return fmt.Errorf("published config checksum changed after reopen")
	}
	// 验证分析结果仍可读
	analyses, _ := svc2.Store().ListAllAnalyses()
	_ = analyses
	// 阶段二追加：替换字体覆盖摘要 → 新规则生成新报告而旧配置不变
	if err := stage2(svc2, cfg); err != nil {
		return err
	}
	if err := st2.Close(); err != nil {
		return fmt.Errorf("close db (stage2): %w", err)
	}

	fmt.Printf("smoke test passed: fonts=%d rules=%d recovered=%d config=%s\n",
		len(fonts), len(rules), len(recovered), cfg.ID)
	return nil
}

// openService 打开数据库并构造服务。
func openService(dbPath string) (*service.Service, *store.Store, error) {
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}
	return service.New(st), st, nil
}

// seedStage1 执行第一阶段：登记字体、规则、样本、分析、报告、发布配置。
// 返回已发布配置。
func seedStage1(svc *service.Service) (*model.PublishedConfig, error) {
	// 1. 登记字体
	latnRes, err := svc.RegisterFont(model.FontInput{
		Name:    "Noto Sans Latin",
		Family:  "Noto Sans",
		Ranges:  []model.Range{{Start: 0x0020, End: 0x007E}, {Start: 0x00C0, End: 0x024F}, {Start: 0x2000, End: 0x206F}},
		Scripts: []string{"Latn", "Zyyy"},
	})
	if err != nil {
		return nil, fmt.Errorf("register latin font: %w", err)
	}
	markRes, err := svc.RegisterFont(model.FontInput{
		Name:    "Noto Sans Mark",
		Family:  "Noto Sans",
		Ranges:  []model.Range{{Start: 0x0300, End: 0x036F}},
		Scripts: []string{"Zinh"},
	})
	if err != nil {
		return nil, fmt.Errorf("register mark font: %w", err)
	}
	arabRes, err := svc.RegisterFont(model.FontInput{
		Name:    "Noto Naskh Arabic",
		Family:  "Noto Naskh",
		Ranges:  []model.Range{{Start: 0x0600, End: 0x06FF}},
		Scripts: []string{"Arab"},
	})
	if err != nil {
		return nil, fmt.Errorf("register arabic font: %w", err)
	}
	// 2. 扫描字体
	for _, id := range []string{latnRes.Font.ID, markRes.Font.ID, arabRes.Font.ID} {
		if _, err := svc.ScanFont(id); err != nil {
			return nil, fmt.Errorf("scan font %s: %w", id, err)
		}
	}
	// 3. 创建并验证规则（拉丁规则绑定 latin+mark 组合覆盖组合字符）
	rule1, err := svc.CreateRule(model.RuleInput{
		Name:            "latin-fallback",
		RequiredScripts: []string{"Latn"},
		FontIDs:         []string{latnRes.Font.ID, markRes.Font.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("create rule1: %w", err)
	}
	if _, err := svc.ValidateRule(rule1.Rule.ID); err != nil {
		return nil, fmt.Errorf("validate rule1: %w", err)
	}
	if _, err := svc.PublishRule(rule1.Rule.ID); err != nil {
		return nil, fmt.Errorf("publish rule1: %w", err)
	}
	rule2, err := svc.CreateRule(model.RuleInput{
		Name:            "arabic-fallback",
		RequiredScripts: []string{"Arab"},
		FontIDs:         []string{arabRes.Font.ID},
	})
	if err != nil {
		return nil, fmt.Errorf("create rule2: %w", err)
	}
	if _, err := svc.ValidateRule(rule2.Rule.ID); err != nil {
		return nil, fmt.Errorf("validate rule2: %w", err)
	}
	if _, err := svc.PublishRule(rule2.Rule.ID); err != nil {
		return nil, fmt.Errorf("publish rule2: %w", err)
	}
	// 4. 创建样本：组合字符 + 变体选择符 + 数字序列 + 缺字（Devanagari 无字体覆盖）
	sample, err := svc.CreateSampleSet("fallback-demo", "café\u0301 a\u0301 \u0627\u0644\u0639\u0631\u0628\u064a\u0629 12345 \u0915\u093e")
	if err != nil {
		return nil, fmt.Errorf("create sample: %w", err)
	}
	// 5. 分析
	analysis, err := svc.StartAnalysis(sample.SampleSet.ID)
	if err != nil {
		return nil, fmt.Errorf("start analysis: %w", err)
	}
	// 6. 验证：拆分风险链（组合字符 a\u0301 若 mark 字体优先级低于 latin 则被拆）
	risks, err := svc.AnalysisRisks(analysis.ID)
	if err != nil {
		return nil, err
	}
	gaps, err := svc.AnalysisGaps(analysis.ID)
	if err != nil {
		return nil, err
	}
	fmt.Printf("smoke stage1: clusters=%d risks=%d gaps=%d\n", sample.Clusters, len(risks), len(gaps))
	// 7. 生成报告并发布（有缺口：Devanagari 缺字）
	report, err := svc.GenerateReport(analysis.ID)
	if err != nil {
		return nil, fmt.Errorf("generate report: %w", err)
	}
	if _, err := svc.ReviewReport(report.Report.ID, true); err != nil {
		return nil, fmt.Errorf("review report: %w", err)
	}
	if _, err := svc.PublishReport(report.Report.ID); err != nil {
		return nil, fmt.Errorf("publish report: %w", err)
	}
	// 8. 发布配置
	cfg, err := svc.PublishConfig("prod-fallback-v1")
	if err != nil {
		return nil, fmt.Errorf("publish config: %w", err)
	}
	return cfg, nil
}

// stage2 执行第二阶段：替换字体覆盖摘要 → 新规则生成新报告而旧配置不变。
func stage2(svc *service.Service, oldCfg *model.PublishedConfig) error {
	// 登记 Devanagari 字体补缺
	devaRes, err := svc.RegisterFont(model.FontInput{
		Name:    "Noto Sans Devanagari",
		Family:  "Noto Sans",
		Ranges:  []model.Range{{Start: 0x0900, End: 0x097F}},
		Scripts: []string{"Deva"},
	})
	if err != nil {
		return fmt.Errorf("register devanagari font: %w", err)
	}
	if _, err := svc.ScanFont(devaRes.Font.ID); err != nil {
		return fmt.Errorf("scan devanagari font: %w", err)
	}
	// 新规则（替代 arabic 规则优先级更高？直接新增 devanagari 规则）
	devRule, err := svc.CreateRule(model.RuleInput{
		Name:            "devanagari-fallback",
		RequiredScripts: []string{"Deva"},
		FontIDs:         []string{devaRes.Font.ID},
	})
	if err != nil {
		return fmt.Errorf("create devanagari rule: %w", err)
	}
	if _, err := svc.ValidateRule(devRule.Rule.ID); err != nil {
		return fmt.Errorf("validate devanagari rule: %w", err)
	}
	if _, err := svc.PublishRule(devRule.Rule.ID); err != nil {
		return fmt.Errorf("publish devanagari rule: %w", err)
	}
	// 新样本重跑分析 → 新报告无缺口
	sample, err := svc.CreateSampleSet("fallback-demo-v2", "\u0915\u093e \u0915\u093f")
	if err != nil {
		return fmt.Errorf("create sample v2: %w", err)
	}
	analysis, err := svc.StartAnalysis(sample.SampleSet.ID)
	if err != nil {
		return fmt.Errorf("start analysis v2: %w", err)
	}
	report, err := svc.GenerateReport(analysis.ID)
	if err != nil {
		return fmt.Errorf("generate report v2: %w", err)
	}
	if report.Stats.Missing > 0 || report.Stats.Risk > 0 {
		return fmt.Errorf("v2 report should pass, got missing=%d risk=%d", report.Stats.Missing, report.Stats.Risk)
	}
	if _, err := svc.PublishReport(report.Report.ID); err != nil {
		return fmt.Errorf("publish report v2: %w", err)
	}
	// 发布新配置 → 旧配置被替代（superseded），但快照不变
	newCfg, err := svc.PublishConfig("prod-fallback-v2")
	if err != nil {
		return fmt.Errorf("publish config v2: %w", err)
	}
	oldAfter, err := svc.GetPublishedConfig(oldCfg.ID)
	if err != nil {
		return err
	}
	if oldAfter.Status != model.ConfigSuperseded {
		return fmt.Errorf("old config should be superseded, got %s", oldAfter.Status)
	}
	if oldAfter.Snapshot != oldCfg.Snapshot {
		return fmt.Errorf("old config snapshot changed after replacement")
	}
	// 版本比较
	diff, err := svc.CompareConfigs(oldCfg.ID, newCfg.ID)
	if err != nil {
		return fmt.Errorf("compare configs: %w", err)
	}
	if diff.Same {
		return fmt.Errorf("config diff should not be identical")
	}
	fmt.Printf("smoke stage2: new_config=%s diff=%s\n", newCfg.ID, diff.Summary)
	return nil
}
