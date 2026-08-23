package service

import (
	"time"

	"task181-fontproof/internal/fallback"
	"task181-fontproof/internal/grapheme"
	"task181-fontproof/internal/model"
	"task181-fontproof/internal/proof"
	"task181-fontproof/internal/typeface"
)

// SampleResult 是样本集创建结果。
type SampleResult struct {
	SampleSet model.SampleSet `json:"sample_set"`
	Clusters  int             `json:"clusters"`
}

// CreateSampleSet 创建样本集并预切分验证文本非空。
func (s *Service) CreateSampleSet(name, content string) (*SampleResult, error) {
	if name == "" {
		return nil, model.EBadRequest("sample set name is required")
	}
	if content == "" {
		return nil, model.EBadRequest("sample content is required")
	}
	clusters := grapheme.Segmenter(content)
	if len(clusters) == 0 {
		return nil, model.EBadRequest("sample content contains no grapheme clusters")
	}
	ss := model.SampleSet{
		ID:        newID("smp"),
		Name:      name,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.st.CreateSampleSet(ss); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "create_sample_set", "sample_set", ss.ID, name)
	return &SampleResult{SampleSet: ss, Clusters: len(clusters)}, nil
}

// ListSampleSets 返回样本集列表。
func (s *Service) ListSampleSets() ([]model.SampleSet, error) {
	return s.st.ListSampleSets()
}

// GetSampleSet 返回样本集详情。
func (s *Service) GetSampleSet(id string) (*model.SampleSet, error) {
	return s.st.GetSampleSet(id)
}

// StartAnalysis 启动字素簇覆盖分析（写入 running 分析 + 执行 + 完成）。
// 若同一样本集已有 running/resumed 分析则复用（不重复启动）。
func (s *Service) StartAnalysis(sampleID string) (*model.Analysis, error) {
	ss, err := s.st.GetSampleSet(sampleID)
	if err != nil {
		return nil, err
	}
	// 检查既有未完成分析
	existing, _ := s.st.ListAnalysesBySample(sampleID)
	for _, a := range existing {
		if a.Status == model.AnalysisRunning || a.Status == model.AnalysisResumed {
			return s.ResolveAnalysis(a.ID)
		}
	}
	ruleVersion, _ := s.st.CurrentRuleVersion()
	a := model.Analysis{
		ID:          newID("anl"),
		SampleSetID: sampleID,
		Status:      model.AnalysisRunning,
		RuleVersion: ruleVersion,
		Stats:       "{}",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := s.st.CreateAnalysis(a); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "start_analysis", "analysis", a.ID, ss.Name)
	return s.ResolveAnalysis(a.ID)
}

// ResolveAnalysis 执行（或继续）一次分析。
// 断点续传：已写入的字素簇跳过，只处理剩余部分；完成后更新状态与统计。
func (s *Service) ResolveAnalysis(analysisID string) (*model.Analysis, error) {
	a, err := s.st.GetAnalysis(analysisID)
	if err != nil {
		return nil, err
	}
	if a.Status == model.AnalysisCompleted {
		return a, nil
	}
	if a.Status == model.AnalysisFailed {
		return nil, model.EInvalidState("analysis failed and cannot be resumed")
	}
	ss, err := s.st.GetSampleSet(a.SampleSetID)
	if err != nil {
		return nil, err
	}
	engine, err := s.buildEngine()
	if err != nil {
		return nil, err
	}
	clusters := grapheme.Segmenter(ss.Content)
	if len(clusters) == 0 {
		_ = s.st.UpdateAnalysisStatus(analysisID, model.AnalysisFailed, "{}")
		return nil, model.EBadRequest("sample content contains no grapheme clusters")
	}
	// 断点：只计算 position >= 已写数量 的簇
	done, _ := s.st.CountGraphemes(analysisID)
	if done > len(clusters) {
		done = len(clusters)
	}
	stats := model.AnalysisStats{Total: len(clusters)}
	// 已有簇计入统计
	if done > 0 {
		if gs, err := s.st.ListGraphemes(analysisID); err == nil {
			for _, g := range gs {
				countStatus(&stats, g.Status)
			}
		}
	}
	var newGraphemes []model.Grapheme
	for idx := done; idx < len(clusters); idx++ {
		cl := clusters[idx]
		res := engine.ProveCluster(cl.Codepoints, cl.IsComposite, int(cl.Kind))
		g := model.Grapheme{
			ID:         newID("grp"),
			AnalysisID: analysisID,
			Position:   idx,
			Text:       cl.Text,
			Codepoints: cl.Codepoints,
			// Recompute at the persistence boundary; this keeps stored analysis
			// metadata correct if cluster construction evolves independently.
			Script:      grapheme.DetectScript(cl.Codepoints),
			Status:      res.Status,
			Chain:       res.Chain,
			RiskReason:  res.Reason,
			IsComposite: cl.IsComposite,
		}
		newGraphemes = append(newGraphemes, g)
		countStatus(&stats, g.Status)
	}
	if len(newGraphemes) > 0 {
		if err := s.st.CreateGraphemes(newGraphemes); err != nil {
			return nil, err
		}
	}
	statsJSON := marshalStats(stats)
	if err := s.st.UpdateAnalysisStatus(analysisID, model.AnalysisCompleted, statsJSON); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "complete_analysis", "analysis", analysisID, proof.Summarize(stats))
	return s.st.GetAnalysis(analysisID)
}

// buildEngine 构建覆盖证明引擎（字体视图 + 规则顺序）。
func (s *Service) buildEngine() (*proof.Engine, error) {
	fonts, _ := s.st.ListFonts()
	ranges, _ := s.st.AllFontRanges()
	view := typeface.NewCoverageView(fonts, ranges)
	rules, _ := s.st.ListRules()
	order := fallback.ResolveOrder(rules)
	fontsByRule, _ := s.allRuleFonts()
	return &proof.Engine{
		View:        view,
		Order:       order,
		FontsByRule: fontsByRule,
	}, nil
}

// GetAnalysis 返回分析详情。
func (s *Service) GetAnalysis(id string) (*model.Analysis, error) {
	return s.st.GetAnalysis(id)
}

// ListAnalysesBySample 返回样本集的分析列表。
func (s *Service) ListAnalysesBySample(sampleID string) ([]model.Analysis, error) {
	return s.st.ListAnalysesBySample(sampleID)
}

// AnalysisGraphemes 返回分析的字素簇（按位置排序）。
func (s *Service) AnalysisGraphemes(id string) ([]model.Grapheme, error) {
	if _, err := s.st.GetAnalysis(id); err != nil {
		return nil, err
	}
	return s.st.ListGraphemes(id)
}

// AnalysisChains 返回分析中所有回退链（fallback/risk/missing 的字素簇）。
func (s *Service) AnalysisChains(id string) ([]model.Grapheme, error) {
	gs, err := s.AnalysisGraphemes(id)
	if err != nil {
		return nil, err
	}
	var out []model.Grapheme
	for _, g := range gs {
		if len(g.Chain) > 0 {
			out = append(out, g)
		}
	}
	return out, nil
}

// AnalysisGaps 返回缺失链（缺字字素簇 + 最短缺失证明）。
func (s *Service) AnalysisGaps(id string) ([]model.Grapheme, error) {
	gs, err := s.AnalysisGraphemes(id)
	if err != nil {
		return nil, err
	}
	return proof.MissingChains(gs), nil
}

// AnalysisRisks 返回拆分风险字素簇。
func (s *Service) AnalysisRisks(id string) ([]model.Grapheme, error) {
	gs, err := s.AnalysisGraphemes(id)
	if err != nil {
		return nil, err
	}
	return proof.RiskClusters(gs), nil
}

// countStatus 累加统计。
func countStatus(stats *model.AnalysisStats, status string) {
	switch status {
	case model.ClusterCovered:
		stats.Covered++
	case model.ClusterFallback:
		stats.Fallback++
	case model.ClusterRisk:
		stats.Risk++
	case model.ClusterMissing:
		stats.Missing++
	}
}
