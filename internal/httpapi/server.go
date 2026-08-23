// Package httpapi 提供 /api JSON 路由与根页面（浏览器工作台入口）。
package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/service"
)

// Server 是 HTTP 服务。
type Server struct {
	svc *service.Service
	mux *http.ServeMux
}

// New 构造 HTTP 服务并注册路由。
func New(svc *service.Service) *Server {
	s := &Server{svc: svc, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回 http.Handler（供 ListenAndServe 与测试使用）。
func (s *Server) Handler() http.Handler {
	return withMiddleware(s.mux)
}

// routes 注册全部路由（Go 1.22+ method 模式）。
func (s *Server) routes() {
	// 字体
	s.mux.HandleFunc("POST /api/fonts", s.handleRegisterFont)
	s.mux.HandleFunc("GET /api/fonts", s.handleListFonts)
	s.mux.HandleFunc("POST /api/fonts/import", s.handleImportFonts)
	s.mux.HandleFunc("GET /api/fonts/{id}", s.handleGetFont)
	s.mux.HandleFunc("GET /api/fonts/{id}/ranges", s.handleFontRanges)
	s.mux.HandleFunc("POST /api/fonts/{id}/scan", s.handleScanFont)
	s.mux.HandleFunc("POST /api/fonts/{id}/disable", s.handleDisableFont)
	s.mux.HandleFunc("POST /api/fonts/{id}/enable", s.handleEnableFont)
	// 规则
	s.mux.HandleFunc("POST /api/rules", s.handleCreateRule)
	s.mux.HandleFunc("GET /api/rules", s.handleListRules)
	s.mux.HandleFunc("POST /api/rules/reorder", s.handleReorderRules)
	s.mux.HandleFunc("GET /api/rules/{id}", s.handleGetRule)
	s.mux.HandleFunc("PUT /api/rules/{id}", s.handleUpdateRule)
	s.mux.HandleFunc("POST /api/rules/{id}/validate", s.handleValidateRule)
	s.mux.HandleFunc("POST /api/rules/{id}/publish", s.handlePublishRule)
	s.mux.HandleFunc("POST /api/rules/{id}/supersede", s.handleSupersedeRule)
	s.mux.HandleFunc("POST /api/rules/{id}/fonts", s.handleBindRuleFonts)
	s.mux.HandleFunc("DELETE /api/rules/{id}/fonts/{fontId}", s.handleUnbindRuleFont)
	s.mux.HandleFunc("GET /api/rules/{id}/cycle-check", s.handleRuleCycleCheck)
	// 样本与分析
	s.mux.HandleFunc("POST /api/samples", s.handleCreateSample)
	s.mux.HandleFunc("GET /api/samples", s.handleListSamples)
	s.mux.HandleFunc("GET /api/samples/{id}", s.handleGetSample)
	s.mux.HandleFunc("POST /api/samples/{id}/analyze", s.handleStartAnalysis)
	s.mux.HandleFunc("GET /api/analyses/{id}", s.handleGetAnalysis)
	s.mux.HandleFunc("GET /api/analyses/{id}/chains", s.handleAnalysisChains)
	s.mux.HandleFunc("GET /api/analyses/{id}/gaps", s.handleAnalysisGaps)
	s.mux.HandleFunc("GET /api/analyses/{id}/risks", s.handleAnalysisRisks)
	// 报告
	s.mux.HandleFunc("POST /api/reports", s.handleGenerateReport)
	s.mux.HandleFunc("GET /api/reports", s.handleListReports)
	s.mux.HandleFunc("GET /api/reports/{id}", s.handleGetReport)
	s.mux.HandleFunc("POST /api/reports/{id}/review", s.handleReviewReport)
	s.mux.HandleFunc("POST /api/reports/{id}/publish", s.handlePublishReport)
	// 配置
	s.mux.HandleFunc("POST /api/configs", s.handlePublishConfig)
	s.mux.HandleFunc("GET /api/configs", s.handleListConfigs)
	s.mux.HandleFunc("GET /api/configs/compare", s.handleCompareConfigs)
	s.mux.HandleFunc("GET /api/configs/{id}", s.handleGetConfig)
	s.mux.HandleFunc("GET /api/configs/{id}/versions", s.handleConfigVersions)
	// 自检与审计
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/selfcheck", s.handleSelfCheck)
	s.mux.HandleFunc("GET /api/audit", s.handleAudit)
	// 根页面
	s.mux.HandleFunc("GET /{$}", s.handleRoot)
}

// ---- 通用响应 ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func ok(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func created(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, map[string]any{"data": data})
}

func fail(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	kind := "internal"
	if de, ok := err.(*model.DomainError); ok {
		kind = de.Kind
		switch {
		case strings.Contains(kind, "not_found"):
			status = http.StatusNotFound
		case strings.Contains(kind, "conflict"), strings.Contains(kind, "cycle"),
			strings.Contains(kind, "mutually_exclusive"):
			status = http.StatusConflict
		case strings.Contains(kind, "forbidden"):
			status = http.StatusForbidden
		case strings.Contains(kind, "invalid_state"):
			status = http.StatusConflict
		case strings.Contains(kind, "bad_request"), strings.Contains(kind, "gapped"):
			status = http.StatusBadRequest
		default:
			status = http.StatusBadRequest
		}
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{"kind": kind, "message": err.Error()}})
}

// bodyJSON 解析请求体 JSON 到目标结构。
func bodyJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

// pathID 读取路径参数。
func pathID(r *http.Request, name string) string {
	return r.PathValue(name)
}

// withMiddleware 包装日志与恢复中间件。
func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				writeJSON(sw, http.StatusInternalServerError, map[string]any{"error": map[string]any{"kind": "panic", "message": fmt.Sprint(rec)}})
			}
			log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
		}()
		next.ServeHTTP(sw, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
