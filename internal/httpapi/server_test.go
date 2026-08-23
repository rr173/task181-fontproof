package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/service"
	"task181-fontproof/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return New(service.New(st))
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	var wrap struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &wrap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wrap.Data, v); err != nil {
		t.Fatal(err)
	}
}

func TestAPIFontLifecycle(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()

	// 登记字体
	rec := doJSON(t, h, http.MethodPost, "/api/fonts", model.FontInput{
		Name: "Noto Sans Latin", Family: "Noto Sans",
		Ranges: []model.Range{{Start: 0x0041, End: 0x007A}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register font: %d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Font model.Font `json:"font"`
	}
	decodeData(t, rec, &res)
	fontID := res.Font.ID

	// 列表
	rec = doJSON(t, h, http.MethodGet, "/api/fonts", nil)
	var fonts []model.Font
	decodeData(t, rec, &fonts)
	if len(fonts) != 1 {
		t.Fatalf("expected 1 font, got %d", len(fonts))
	}

	// 扫描
	rec = doJSON(t, h, http.MethodPost, "/api/fonts/"+fontID+"/scan", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("scan font: %d %s", rec.Code, rec.Body.String())
	}

	// 停用
	rec = doJSON(t, h, http.MethodPost, "/api/fonts/"+fontID+"/disable", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable font: %d %s", rec.Code, rec.Body.String())
	}

	// 健康与自检
	rec = doJSON(t, h, http.MethodGet, "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health: %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodGet, "/api/selfcheck", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("selfcheck: %d %s", rec.Code, rec.Body.String())
	}

	// 根页面
	rec = doJSON(t, h, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK || len(rec.Body.Bytes()) < 100 {
		t.Fatalf("root page: %d", rec.Code)
	}
}

func TestAPIFullFlow(t *testing.T) {
	s := newTestServer(t)
	h := s.Handler()

	// 字体
	var fonts []struct {
		Font model.Font `json:"font"`
	}
	for _, in := range []model.FontInput{
		{Name: "Latin", Family: "F", Ranges: []model.Range{{Start: 0x0020, End: 0x007E}}, Scripts: []string{"Latn"}},
		{Name: "Mark", Family: "F", Ranges: []model.Range{{Start: 0x0300, End: 0x036F}}, Scripts: []string{"Zinh"}},
	} {
		var res struct {
			Font model.Font `json:"font"`
		}
		decodeData(t, doJSON(t, h, http.MethodPost, "/api/fonts", in), &res)
		fonts = append(fonts, res)
		doJSON(t, h, http.MethodPost, "/api/fonts/"+res.Font.ID+"/scan", nil)
	}

	// 规则
	var ruleRes struct {
		Rule model.FallbackRule `json:"rule"`
	}
	decodeData(t, doJSON(t, h, http.MethodPost, "/api/rules", model.RuleInput{
		Name: "latin", RequiredScripts: []string{"Latn"}, FontIDs: []string{fonts[0].Font.ID, fonts[1].Font.ID},
	}), &ruleRes)
	doJSON(t, h, http.MethodPost, "/api/rules/"+ruleRes.Rule.ID+"/validate", nil)
	doJSON(t, h, http.MethodPost, "/api/rules/"+ruleRes.Rule.ID+"/publish", nil)

	// 样本 + 分析
	var sampleRes struct {
		SampleSet model.SampleSet `json:"sample_set"`
	}
	decodeData(t, doJSON(t, h, http.MethodPost, "/api/samples", map[string]string{"name": "s", "content": "a\u0301"}), &sampleRes)
	var analysis model.Analysis
	decodeData(t, doJSON(t, h, http.MethodPost, "/api/samples/"+sampleRes.SampleSet.ID+"/analyze", nil), &analysis)

	// 风险链
	rec := doJSON(t, h, http.MethodGet, "/api/analyses/"+analysis.ID+"/risks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("risks: %d %s", rec.Code, rec.Body.String())
	}

	// 报告
	var reportRes struct {
		Report model.CoverageReport `json:"report"`
	}
	decodeData(t, doJSON(t, h, http.MethodPost, "/api/reports", map[string]string{"analysis_id": analysis.ID}), &reportRes)

	// 发布配置
	var cfg model.PublishedConfig
	decodeData(t, doJSON(t, h, http.MethodPost, "/api/configs", map[string]string{"name": "prod"}), &cfg)
	if cfg.Status != model.ConfigActive {
		t.Fatalf("expected active config, got %s", cfg.Status)
	}
}
