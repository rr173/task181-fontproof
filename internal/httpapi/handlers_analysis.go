package httpapi

import (
	"net/http"

	"task181-fontproof/internal/model"
)

// ---- 样本 handlers ----

// CreateSampleRequest 创建样本集载荷。
type CreateSampleRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (s *Server) handleCreateSample(w http.ResponseWriter, r *http.Request) {
	var in CreateSampleRequest
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	res, err := s.svc.CreateSampleSet(in.Name, in.Content)
	if err != nil {
		fail(w, err)
		return
	}
	created(w, res)
}

func (s *Server) handleListSamples(w http.ResponseWriter, r *http.Request) {
	samples, err := s.svc.ListSampleSets()
	if err != nil {
		fail(w, err)
		return
	}
	if samples == nil {
		samples = []model.SampleSet{}
	}
	ok(w, samples)
}

func (s *Server) handleGetSample(w http.ResponseWriter, r *http.Request) {
	ss, err := s.svc.GetSampleSet(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	analyses, _ := s.svc.ListAnalysesBySample(ss.ID)
	if analyses == nil {
		analyses = []model.Analysis{}
	}
	ok(w, map[string]any{"sample_set": ss, "analyses": analyses})
}

func (s *Server) handleStartAnalysis(w http.ResponseWriter, r *http.Request) {
	a, err := s.svc.StartAnalysis(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, a)
}

// ---- 分析 handlers ----

func (s *Server) handleGetAnalysis(w http.ResponseWriter, r *http.Request) {
	a, err := s.svc.GetAnalysis(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	graphemes, _ := s.svc.AnalysisGraphemes(a.ID)
	if graphemes == nil {
		graphemes = []model.Grapheme{}
	}
	ok(w, map[string]any{"analysis": a, "graphemes": graphemes})
}

func (s *Server) handleAnalysisChains(w http.ResponseWriter, r *http.Request) {
	chains, err := s.svc.AnalysisChains(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	if chains == nil {
		chains = []model.Grapheme{}
	}
	ok(w, chains)
}

func (s *Server) handleAnalysisGaps(w http.ResponseWriter, r *http.Request) {
	gaps, err := s.svc.AnalysisGaps(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	if gaps == nil {
		gaps = []model.Grapheme{}
	}
	ok(w, gaps)
}

func (s *Server) handleAnalysisRisks(w http.ResponseWriter, r *http.Request) {
	risks, err := s.svc.AnalysisRisks(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	if risks == nil {
		risks = []model.Grapheme{}
	}
	ok(w, risks)
}

// ---- 报告 handlers ----

// GenerateReportRequest 生成报告载荷。
type GenerateReportRequest struct {
	AnalysisID string `json:"analysis_id"`
}

func (s *Server) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	var in GenerateReportRequest
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	res, err := s.svc.GenerateReport(in.AnalysisID)
	if err != nil {
		fail(w, err)
		return
	}
	created(w, res)
}

func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	reports, err := s.svc.ListReports()
	if err != nil {
		fail(w, err)
		return
	}
	if reports == nil {
		reports = []model.CoverageReport{}
	}
	ok(w, reports)
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.GetReport(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, res)
}

// ReviewReportRequest 复核载荷。
type ReviewReportRequest struct {
	Approve bool `json:"approve"`
}

func (s *Server) handleReviewReport(w http.ResponseWriter, r *http.Request) {
	var in ReviewReportRequest
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	res, err := s.svc.ReviewReport(pathID(r, "id"), in.Approve)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, res)
}

func (s *Server) handlePublishReport(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.PublishReport(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, res)
}

// ---- 配置 handlers ----

// PublishConfigRequest 发布配置载荷。
type PublishConfigRequest struct {
	Name string `json:"name"`
}

func (s *Server) handlePublishConfig(w http.ResponseWriter, r *http.Request) {
	var in PublishConfigRequest
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	cfg, err := s.svc.PublishConfig(in.Name)
	if err != nil {
		fail(w, err)
		return
	}
	created(w, cfg)
}

func (s *Server) handleListConfigs(w http.ResponseWriter, r *http.Request) {
	cfgs, err := s.svc.ListPublishedConfigs()
	if err != nil {
		fail(w, err)
		return
	}
	if cfgs == nil {
		cfgs = []model.PublishedConfig{}
	}
	ok(w, cfgs)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.svc.GetPublishedConfig(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, cfg)
}

func (s *Server) handleConfigVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := s.svc.ListConfigVersions(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	if versions == nil {
		versions = []model.ConfigVersion{}
	}
	ok(w, versions)
}

func (s *Server) handleCompareConfigs(w http.ResponseWriter, r *http.Request) {
	base := r.URL.Query().Get("base")
	target := r.URL.Query().Get("target")
	if base == "" || target == "" {
		fail(w, model.EBadRequest("query params base and target are required"))
		return
	}
	diff, err := s.svc.CompareConfigs(base, target)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, diff)
}

// ---- 自检与审计 handlers ----

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]any{"status": "ok", "service": "task181-fontproof"})
}

func (s *Server) handleSelfCheck(w http.ResponseWriter, r *http.Request) {
	ok(w, s.svc.SelfCheck())
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	events, err := s.svc.Store().ListAudit(100)
	if err != nil {
		fail(w, err)
		return
	}
	if events == nil {
		events = []model.AuditEvent{}
	}
	ok(w, events)
}
