package service

import (
	"time"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/proof"
	"task181-fontproof/internal/release"
	"task181-fontproof/internal/store"
	"task181-fontproof/internal/typeface"
)

// ReportResult 是报告操作结果。
type ReportResult struct {
	Report model.CoverageReport `json:"report"`
	Stats  model.AnalysisStats  `json:"stats"`
}

// GenerateReport 从一次已完成的覆盖分析生成覆盖报告（generating → pending_review）。
func (s *Service) GenerateReport(analysisID string) (*ReportResult, error) {
	a, err := s.st.GetAnalysis(analysisID)
	if err != nil {
		return nil, err
	}
	if a.Status != model.AnalysisCompleted {
		return nil, model.EInvalidState("analysis must be completed before generating a report")
	}
	// 检查是否已有该分析的报告（幂等）
	reports, _ := s.st.ListReports()
	for _, r := range reports {
		if r.AnalysisID == analysisID && r.Status != model.ReportPublished {
			// 已有未发布报告则返回既有
			stats := store.UnmarshalStats(r.Stats)
			return &ReportResult{Report: r, Stats: stats}, nil
		}
	}
	stats := store.UnmarshalStats(a.Stats)
	rep := model.CoverageReport{
		ID:          newID("rpt"),
		AnalysisID:  analysisID,
		RuleVersion: a.RuleVersion,
		Status:      model.ReportGenerating,
		Stats:       a.Stats,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := s.st.CreateReport(rep); err != nil {
		return nil, err
	}
	// 立即完成生成：判定 through / gapped
	status := model.ReportPendingReview
	if proof.Passed(stats) {
		status = model.ReportPassed
	}
	if err := s.st.UpdateReportStatus(rep.ID, status, ""); err != nil {
		return nil, err
	}
	rep.Status = status
	s.addAudit("engineer", "generate_report", "report", rep.ID, proof.Summarize(stats))
	return &ReportResult{Report: rep, Stats: stats}, nil
}

// ReviewReport 复核报告：通过/有缺口（pending_review → passed / gapped）。
func (s *Service) ReviewReport(id string, approve bool) (*ReportResult, error) {
	r, err := s.st.GetReport(id)
	if err != nil {
		return nil, err
	}
	if r.Status != model.ReportPendingReview && r.Status != model.ReportPassed && r.Status != model.ReportGapped {
		return nil, model.EInvalidState("only pending/passed/gapped reports can be reviewed")
	}
	status := model.ReportGapped
	if approve {
		status = model.ReportPassed
	}
	if err := s.st.UpdateReportStatus(id, status, ""); err != nil {
		return nil, err
	}
	r.Status = status
	s.addAudit("reviewer", "review_report", "report", id, status)
	stats := store.UnmarshalStats(r.Stats)
	return &ReportResult{Report: *r, Stats: stats}, nil
}

// PublishReport 发布报告（passed/gapped → published）。
func (s *Service) PublishReport(id string) (*ReportResult, error) {
	r, err := s.st.GetReport(id)
	if err != nil {
		return nil, err
	}
	if r.Status != model.ReportPassed && r.Status != model.ReportGapped {
		return nil, model.EInvalidState("only passed/gapped reports can be published")
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
	if err := s.st.UpdateReportStatus(id, model.ReportPublished, now); err != nil {
		return nil, err
	}
	r.Status = model.ReportPublished
	s.addAudit("reviewer", "publish_report", "report", id, "")
	stats := store.UnmarshalStats(r.Stats)
	return &ReportResult{Report: *r, Stats: stats}, nil
}

// ListReports 返回全部报告。
func (s *Service) ListReports() ([]model.CoverageReport, error) {
	return s.st.ListReports()
}

// GetReport 返回报告详情。
func (s *Service) GetReport(id string) (*ReportResult, error) {
	r, err := s.st.GetReport(id)
	if err != nil {
		return nil, err
	}
	return &ReportResult{Report: *r, Stats: store.UnmarshalStats(r.Stats)}, nil
}

// PublishConfig 发布回退配置：冻结当前规则快照与字体覆盖摘要。
// 同一 rule_version 已存在 active 配置时视为替代（supersede）旧配置。
func (s *Service) PublishConfig(name string) (*model.PublishedConfig, error) {
	if name == "" {
		return nil, model.EBadRequest("config name is required")
	}
	ruleVersion, _ := s.st.CurrentRuleVersion()
	checksumState, err := s.RuleSetChecksum()
	if err != nil {
		return nil, err
	}
	snap, err := s.buildSnapshot(ruleVersion)
	if err != nil {
		return nil, err
	}
	res, err := release.NewPublishedConfig(newID("cfg"), name, ruleVersion, checksumState.Checksum, snap)
	if err != nil {
		return nil, err
	}
	// 替代同版本既有 active 配置
	existing, _ := s.st.ListPublishedConfigs()
	for _, c := range existing {
		if c.Status == model.ConfigActive {
			if err := s.st.SupersedePublishedConfig(c.ID, res.Config.ID); err != nil {
				return nil, err
			}
		}
	}
	if err := s.st.CreatePublishedConfig(res.Config); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "publish_config", "config", res.Config.ID, name)
	return s.st.GetPublishedConfig(res.Config.ID)
}

// ListPublishedConfigs 返回全部已发布配置。
func (s *Service) ListPublishedConfigs() ([]model.PublishedConfig, error) {
	return s.st.ListPublishedConfigs()
}

// GetPublishedConfig 返回配置详情。
func (s *Service) GetPublishedConfig(id string) (*model.PublishedConfig, error) {
	return s.st.GetPublishedConfig(id)
}

// CompareConfigs 比较两个配置版本。
func (s *Service) CompareConfigs(baseID, targetID string) (*model.ConfigDiff, error) {
	base, err := s.st.GetPublishedConfig(baseID)
	if err != nil {
		return nil, err
	}
	target, err := s.st.GetPublishedConfig(targetID)
	if err != nil {
		return nil, err
	}
	baseSnap, err := release.Unmarshal(base.Snapshot)
	if err != nil {
		return nil, err
	}
	targetSnap, err := release.Unmarshal(target.Snapshot)
	if err != nil {
		return nil, err
	}
	diff := release.Compare(baseSnap, targetSnap)
	return &diff, nil
}

// ListConfigVersions 返回配置版本历史。
func (s *Service) ListConfigVersions(configID string) ([]model.ConfigVersion, error) {
	return s.st.ListConfigVersions(configID)
}

// buildSnapshot 构建冻结快照。
func (s *Service) buildSnapshot(ruleVersion int) (*release.Snapshot, error) {
	rules, _ := s.st.ListRules()
	fontsByRule, _ := s.allRuleFonts()
	fonts, _ := s.st.ListFonts()
	rangeDesc := map[string]string{}
	for _, f := range fonts {
		rs, _ := s.st.FontRanges(f.ID)
		var norm []model.Range
		for _, rr := range rs {
			norm = append(norm, model.Range{Start: rr.Start, End: rr.End})
		}
		rangeDesc[f.ID] = typeface.DescribeRanges(norm)
	}
	return release.BuildSnapshot(ruleVersion, rules, fontsByRule, fonts, rangeDesc)
}

// SelfCheck 执行自检：数据库连通、字素引擎、规则引擎、证明引擎。
func (s *Service) SelfCheck() map[string]string {
	out := map[string]string{}
	// 1. 数据库
	if _, err := s.st.ListFonts(); err != nil {
		out["store"] = "fail: " + err.Error()
	} else {
		out["store"] = "pass"
	}
	// 2. 字素引擎
	clusters := []rune("a\u0301e\u0301") // á é
	if len(clusters) == 2 {
		out["grapheme"] = "pass"
	} else {
		out["grapheme"] = "fail"
	}
	// 3. 规则引擎（循环检测）
	if err := s.CheckRuleCycle(); err != nil {
		out["fallback"] = "fail: " + err.Error()
	} else {
		out["fallback"] = "pass"
	}
	// 4. 证明引擎
	engine, err := s.buildEngine()
	if err != nil {
		out["proof"] = "fail: " + err.Error()
	} else {
		if engine == nil {
			out["proof"] = "fail: engine nil"
		} else {
			out["proof"] = "pass"
		}
	}
	// 5. 发布引擎（快照构建）
	if _, err := s.buildSnapshot(0); err != nil {
		out["release"] = "fail: " + err.Error()
	} else {
		out["release"] = "pass"
	}
	return out
}
