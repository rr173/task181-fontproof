package httpapi

import (
	"net/http"
	"time"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/service"
)

// ---- 字体 handlers ----

func (s *Server) handleRegisterFont(w http.ResponseWriter, r *http.Request) {
	var in model.FontInput
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	res, err := s.svc.RegisterFont(in)
	if err != nil {
		fail(w, err)
		return
	}
	created(w, res)
}

func (s *Server) handleListFonts(w http.ResponseWriter, r *http.Request) {
	fonts, err := s.svc.ListFonts()
	if err != nil {
		fail(w, err)
		return
	}
	if fonts == nil {
		fonts = []model.Font{}
	}
	ok(w, fonts)
}

// ImportFontsRequest 批量导入字体摘要。
type ImportFontsRequest struct {
	Fonts []model.FontInput `json:"fonts"`
}

func (s *Server) handleImportFonts(w http.ResponseWriter, r *http.Request) {
	var in ImportFontsRequest
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	var out []*struct {
		Font   model.Font `json:"font"`
		Reused bool       `json:"reused"`
	}
	for _, f := range in.Fonts {
		res, err := s.svc.RegisterFont(f)
		if err != nil {
			fail(w, err)
			return
		}
		out = append(out, &struct {
			Font   model.Font `json:"font"`
			Reused bool       `json:"reused"`
		}{Font: res.Font, Reused: res.Reused})
	}
	ok(w, out)
}

func (s *Server) handleGetFont(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.GetFontDetail(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, res)
}

func (s *Server) handleFontRanges(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.GetFontDetail(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, res.Ranges)
}

func (s *Server) handleScanFont(w http.ResponseWriter, r *http.Request) {
	f, err := s.svc.ScanFont(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, f)
}

func (s *Server) handleDisableFont(w http.ResponseWriter, r *http.Request) {
	f, err := s.svc.DisableFont(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, f)
}

func (s *Server) handleEnableFont(w http.ResponseWriter, r *http.Request) {
	f, err := s.svc.EnableFont(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, f)
}

// ---- 规则 handlers ----

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var in model.RuleInput
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	d, err := s.svc.CreateRule(in)
	if err != nil {
		fail(w, err)
		return
	}
	created(w, d)
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.svc.ListRules()
	if err != nil {
		fail(w, err)
		return
	}
	if rules == nil {
		rules = []service.RuleDetail{}
	}
	ok(w, rules)
}

// UpdateRuleRequest 编辑规则的载荷（含乐观锁版本）。
type UpdateRuleRequest struct {
	model.RuleInput
	ExpectedVersion int `json:"expected_version"`
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	var in UpdateRuleRequest
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	d, err := s.svc.UpdateRule(pathID(r, "id"), in.RuleInput, in.ExpectedVersion)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, d)
}

func (s *Server) handleGetRule(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.GetRuleDetail(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, d)
}

func (s *Server) handleValidateRule(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.ValidateRule(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, d)
}

func (s *Server) handlePublishRule(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.PublishRule(pathID(r, "id"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, d)
}

func (s *Server) handleSupersedeRule(w http.ResponseWriter, r *http.Request) {
	var in model.RuleInput
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	d, err := s.svc.SupersedeRule(pathID(r, "id"), in)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, d)
}

// BindFontsRequest 绑定字体载荷。
type BindFontsRequest struct {
	FontIDs []string `json:"font_ids"`
}

func (s *Server) handleBindRuleFonts(w http.ResponseWriter, r *http.Request) {
	var in BindFontsRequest
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	d, err := s.svc.BindRuleFonts(pathID(r, "id"), in.FontIDs)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, d)
}

func (s *Server) handleUnbindRuleFont(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.UnbindRuleFont(pathID(r, "id"), pathID(r, "fontId"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, d)
}

// ReorderRequest 重排载荷（含 checksum 冲突检测）。
type ReorderRequest struct {
	NewOrder       []string `json:"new_order"`
	ExpectChecksum string   `json:"expect_checksum"`
}

func (s *Server) handleReorderRules(w http.ResponseWriter, r *http.Request) {
	var in ReorderRequest
	if err := bodyJSON(r, &in); err != nil {
		fail(w, model.EBadRequest("invalid request body: "+err.Error()))
		return
	}
	res, err := s.svc.ReorderRules(in.NewOrder, in.ExpectChecksum)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, res)
}

func (s *Server) handleRuleCycleCheck(w http.ResponseWriter, r *http.Request) {
	// 校验规则集整体循环（{id} 参数仅作定位）
	if _, err := s.svc.GetRuleDetail(pathID(r, "id")); err != nil {
		fail(w, err)
		return
	}
	if err := s.svc.CheckRuleCycle(); err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]bool{"cycle_free": true})
}

// ---- 根页面 ----

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>数字字体回退覆盖证明工作台</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;max-width:960px;margin:40px auto;padding:0 20px;color:#1f2937;line-height:1.7}
h1{font-size:1.6rem;border-bottom:2px solid #e5e7eb;padding-bottom:12px}
code{background:#f3f4f6;padding:2px 6px;border-radius:4px;font-size:.9em}
table{border-collapse:collapse;width:100%;margin:16px 0}
th,td{border:1px solid #e5e7eb;padding:8px 12px;text-align:left;font-size:.9rem}
th{background:#f9fafb}
.badge{display:inline-block;background:#dbeafe;color:#1e40af;border-radius:999px;padding:2px 10px;font-size:.8rem}
</style></head><body>
<h1>数字字体回退覆盖证明工作台 <span class="badge">API service</span></h1>
<p>字体工程师在此审查一组字体回退规则是否完整覆盖目标语言集合，并找出会把组合字符、变体选择符或数字系统错误拆分的最短回退链。全部能力通过 <code>/api</code> JSON 接口提供；本页面只展示入口与常用命令。</p>
<h2>核心闭环</h2>
<ol><li>登记字体覆盖摘要（Unicode 范围、特性标签、脚本）→ 扫描评估可用/冲突</li>
<li>创建回退规则并绑定字体（优先级 + rank 顺序）→ 验证必需脚本覆盖</li>
<li>提交文本样本 → 字素簇切分 → 覆盖分析（选择链 / 拆分风险 / 最短缺失链）</li>
<li>生成覆盖报告 → 复核通过/有缺口 → 发布冻结配置 → 版本比较</li></ol>
<h2>常用命令（JSON，均以 /api 为前缀）</h2>
<table>
<tr><th>能力</th><th>API</th></tr>
<tr><td>登记字体</td><td><code>POST /api/fonts</code></td></tr>
<tr><td>扫描字体</td><td><code>POST /api/fonts/{id}/scan</code></td></tr>
<tr><td>创建规则</td><td><code>POST /api/rules</code></td></tr>
<tr><td>验证规则</td><td><code>POST /api/rules/{id}/validate</code></td></tr>
<tr><td>发布规则</td><td><code>POST /api/rules/{id}/publish</code></td></tr>
<tr><td>创建样本</td><td><code>POST /api/samples</code></td></tr>
<tr><td>覆盖分析</td><td><code>POST /api/samples/{id}/analyze</code></td></tr>
<tr><td>缺失链证明</td><td><code>GET /api/analyses/{id}/gaps</code></td></tr>
<tr><td>拆分风险</td><td><code>GET /api/analyses/{id}/risks</code></td></tr>
<tr><td>生成报告</td><td><code>POST /api/reports</code></td></tr>
<tr><td>发布配置</td><td><code>POST /api/configs</code></td></tr>
<tr><td>版本比较</td><td><code>GET /api/configs/compare?base={a}&amp;target={b}</code></td></tr>
<tr><td>自检</td><td><code>GET /api/selfcheck</code></td></tr>
</table>
<p style="color:#6b7280;font-size:.85rem">server time: ` + now + ` · smoke test：<code>go run ./cmd/fontproof --smoke-test</code></p>
</body></html>`))
}
