package service

import (
	"time"

	"task181-fontproof/internal/fallback"
	"task181-fontproof/internal/model"
	"task181-fontproof/internal/typeface"
)

// RuleDetail 是规则及其绑定字体的完整视图。
type RuleDetail struct {
	Rule      model.FallbackRule `json:"rule"`
	FontIDs   []string           `json:"font_ids"`
	FontNames []string           `json:"font_names"`
	Checksum  string             `json:"checksum"`
}

// RuleSetState 是当前规则集的汇总（checksum + 版本）。
type RuleSetState struct {
	Checksum string `json:"checksum"`
	Version  int    `json:"version"`
}

// CreateRule 创建回退规则（默认 priority 为当前最大值 +1，避免并发冲突）。
func (s *Service) CreateRule(in model.RuleInput) (*RuleDetail, error) {
	if in.Name == "" {
		return nil, model.EBadRequest("rule name is required")
	}
	rules, _ := s.st.ListRules()
	priority := in.Priority
	if priority <= 0 {
		priority = len(rules) + 1
		for _, r := range rules {
			if r.Priority >= priority {
				priority = r.Priority + 1
			}
		}
	}
	// 同优先级互斥检查（针对已有规则）
	for _, existing := range rules {
		if existing.Priority != priority {
			continue
		}
		existingFonts, _ := s.st.RuleFonts(existing.ID)
		exFonts := make([]string, 0, len(existingFonts))
		for _, rf := range existingFonts {
			exFonts = append(exFonts, rf.FontID)
		}
		if err := fallback.CheckMutualExclusion(existing, model.FallbackRule{ID: "new", Priority: priority, RequiredScripts: in.RequiredScripts}, exFonts, in.FontIDs); err != nil {
			return nil, err
		}
	}
	r := model.FallbackRule{
		ID:              newID("rul"),
		Name:            in.Name,
		Priority:        priority,
		Status:          model.RuleDraft,
		RequiredScripts: uniqueStrings(in.RequiredScripts),
		Version:         1,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	// 校验绑定字体存在
	for _, fontID := range in.FontIDs {
		if _, err := s.st.GetFont(fontID); err != nil {
			return nil, model.EBadRequest("bound font not found: " + fontID)
		}
	}
	var ruleFonts []model.RuleFont
	for i, fontID := range in.FontIDs {
		ruleFonts = append(ruleFonts, model.RuleFont{RuleID: r.ID, FontID: fontID, Rank: i})
	}
	// 计算 checksum（基于全部规则）
	ruleFontsAll, _ := s.allRuleFonts()
	ruleFontsAll[r.ID] = in.FontIDs
	r.Checksum = fallback.Checksum(append(rules, r), ruleFontsAll)
	if err := s.st.CreateRule(r, ruleFonts); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "create_rule", "rule", r.ID, r.Name)
	return s.ruleDetail(r.ID)
}

// UpdateRule 编辑未发布规则（乐观锁：expectedVersion）。
func (s *Service) UpdateRule(id string, in model.RuleInput, expectedVersion int) (*RuleDetail, error) {
	r, err := s.st.GetRule(id)
	if err != nil {
		return nil, err
	}
	if r.Status == model.RulePublished {
		return nil, model.EForbidden("published rule cannot be edited directly; supersede it instead")
	}
	if r.Status == model.RuleSuperseded {
		return nil, model.EInvalidState("superseded rule cannot be edited")
	}
	for _, fontID := range in.FontIDs {
		if _, err := s.st.GetFont(fontID); err != nil {
			return nil, model.EBadRequest("bound font not found: " + fontID)
		}
	}
	r.Name = in.Name
	r.RequiredScripts = uniqueStrings(in.RequiredScripts)
	r.Version++
	r.UpdatedAt = time.Now().UTC()
	// 重新计算 checksum
	rules, _ := s.st.ListRules()
	ruleFontsAll, _ := s.allRuleFonts()
	ruleFontsAll[id] = in.FontIDs
	r.Checksum = fallback.Checksum(rules, ruleFontsAll)
	if err := s.st.UpdateRule(*r, expectedVersion); err != nil {
		return nil, err
	}
	var ruleFonts []model.RuleFont
	for i, fontID := range in.FontIDs {
		ruleFonts = append(ruleFonts, model.RuleFont{RuleID: id, FontID: fontID, Rank: i})
	}
	if err := s.st.SetRuleFonts(id, ruleFonts); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "update_rule", "rule", id, "")
	return s.ruleDetail(id)
}

// ValidateRule 验证规则：必需脚本覆盖判定 → verifiable / gapped。
func (s *Service) ValidateRule(id string) (*RuleDetail, error) {
	r, err := s.st.GetRule(id)
	if err != nil {
		return nil, err
	}
	if r.Status != model.RuleDraft && r.Status != model.RuleVerifiable && r.Status != model.RuleGapped {
		return nil, model.EInvalidState("only draft/verifiable/gapped rules can be validated")
	}
	snap, err := s.ruleSnapshot(id)
	if err != nil {
		return nil, err
	}
	view := s.coverageView()
	vr := fallback.ValidateRule(snap, view)
	status := vr.Status
	if err := s.st.UpdateRuleStatus(id, status, r.Version+1); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "validate_rule", "rule", id, joinReasons(vr.Reasons))
	return s.ruleDetail(id)
}

// PublishRule 发布规则（draft/verifiable/gapped → published）。
// 发布前强制验证；gapped 规则不允许发布。
func (s *Service) PublishRule(id string) (*RuleDetail, error) {
	r, err := s.st.GetRule(id)
	if err != nil {
		return nil, err
	}
	if r.Status == model.RulePublished {
		return nil, model.EInvalidState("rule already published")
	}
	if r.Status == model.RuleSuperseded {
		return nil, model.EInvalidState("superseded rule cannot be published")
	}
	snap, err := s.ruleSnapshot(id)
	if err != nil {
		return nil, err
	}
	view := s.coverageView()
	vr := fallback.ValidateRule(snap, view)
	if vr.Status == model.RuleGapped {
		return nil, model.E("gapped", model.ErrMissingScript, "cannot publish gapped rule: "+joinReasons(vr.Reasons))
	}
	// 循环检测
	ruleFontsAll, _ := s.allRuleFonts()
	if err := fallback.CheckCycle(ruleFontsAll); err != nil {
		return nil, err
	}
	if err := s.st.UpdateRuleStatus(id, model.RulePublished, r.Version+1); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "publish_rule", "rule", id, "")
	return s.ruleDetail(id)
}

// SupersedeRule 替代已发布规则（published → superseded），创建新规则承接。
func (s *Service) SupersedeRule(id string, in model.RuleInput) (*RuleDetail, error) {
	r, err := s.st.GetRule(id)
	if err != nil {
		return nil, err
	}
	if r.Status != model.RulePublished {
		return nil, model.EInvalidState("only published rules can be superseded")
	}
	if err := s.st.SetRuleSupersession(id, "pending"); err != nil {
		return nil, err
	}
	// 新规则继承绑定与脚本，允许更新字体
	fonts, _ := s.st.RuleFonts(id)
	fontIDs := make([]string, 0, len(fonts))
	for _, rf := range fonts {
		fontIDs = append(fontIDs, rf.FontID)
	}
	if in.FontIDs != nil {
		fontIDs = in.FontIDs
	}
	req := in.RequiredScripts
	if req == nil {
		req = r.RequiredScripts
	}
	detail, err := s.CreateRule(model.RuleInput{
		Name:            in.Name,
		Priority:        r.Priority,
		RequiredScripts: req,
		FontIDs:         fontIDs,
	})
	if err != nil {
		return nil, err
	}
	// 记录替代关系
	if err := s.st.SetRuleSupersession(id, detail.Rule.ID); err != nil {
		return nil, err
	}
	// 更新新规则的 supersedes
	rules, _ := s.st.ListRules()
	for i := range rules {
		if rules[i].ID == detail.Rule.ID {
			rules[i].Supersedes = id
			// 用 UpdateRule 写回
			_ = s.st.UpdateRule(rules[i], -1)
			break
		}
	}
	s.addAudit("engineer", "supersede_rule", "rule", id, "superseded by "+detail.Rule.ID)
	return detail, nil
}

// BindRuleFonts 为规则绑定字体（替换既有绑定）。
func (s *Service) BindRuleFonts(id string, fontIDs []string) (*RuleDetail, error) {
	r, err := s.st.GetRule(id)
	if err != nil {
		return nil, err
	}
	if r.Status == model.RulePublished {
		return nil, model.EForbidden("published rule cannot be modified; supersede it instead")
	}
	for _, fontID := range fontIDs {
		if _, err := s.st.GetFont(fontID); err != nil {
			return nil, model.EBadRequest("bound font not found: " + fontID)
		}
	}
	var ruleFonts []model.RuleFont
	for i, fontID := range fontIDs {
		ruleFonts = append(ruleFonts, model.RuleFont{RuleID: id, FontID: fontID, Rank: i})
	}
	if err := s.st.SetRuleFonts(id, ruleFonts); err != nil {
		return nil, err
	}
	// 更新 checksum
	rules, _ := s.st.ListRules()
	ruleFontsAll, _ := s.allRuleFonts()
	ruleFontsAll[id] = fontIDs
	r.Checksum = fallback.Checksum(rules, ruleFontsAll)
	r.Version++
	r.UpdatedAt = time.Now().UTC()
	_ = s.st.UpdateRule(*r, -1)
	s.addAudit("engineer", "bind_rule_fonts", "rule", id, "")
	return s.ruleDetail(id)
}

// UnbindRuleFont 解绑规则下的字体。
func (s *Service) UnbindRuleFont(id, fontID string) (*RuleDetail, error) {
	r, err := s.st.GetRule(id)
	if err != nil {
		return nil, err
	}
	if r.Status == model.RulePublished {
		return nil, model.EForbidden("published rule cannot be modified; supersede it instead")
	}
	if err := s.st.DeleteRuleFont(id, fontID); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "unbind_rule_font", "rule", id, fontID)
	return s.ruleDetail(id)
}

// ReorderRules 重排规则优先级（checksum 冲突检测）。
func (s *Service) ReorderRules(newOrder []string, expectChecksum string) (*RuleSetState, error) {
	rules, _ := s.st.ListRules()
	ruleFontsAll, _ := s.allRuleFonts()
	res, err := fallback.ApplyReorder(rules, ruleFontsAll, newOrder, expectChecksum)
	if err != nil {
		return nil, err
	}
	for _, r := range res.Rules {
		if err := s.st.UpdateRuleStatus(r.ID, r.Status, r.Version+1); err != nil {
			return nil, err
		}
		// 写回 priority
		if err := s.st.UpdateRulePriority(r.ID, r.Priority); err != nil {
			return nil, err
		}
	}
	s.addAudit("engineer", "reorder_rules", "rule_set", "", "")
	return &RuleSetState{Checksum: res.Checksum, Version: res.Version}, nil
}

// RuleSetChecksum 返回当前规则集 checksum 与版本。
func (s *Service) RuleSetChecksum() (*RuleSetState, error) {
	rules, _ := s.st.ListRules()
	ruleFontsAll, _ := s.allRuleFonts()
	return &RuleSetState{
		Checksum: fallback.Checksum(rules, ruleFontsAll),
		Version:  len(rules),
	}, nil
}

// ListRules 返回全部规则详情。
func (s *Service) ListRules() ([]RuleDetail, error) {
	rules, _ := s.st.ListRules()
	out := make([]RuleDetail, 0, len(rules))
	for _, r := range rules {
		d, err := s.ruleDetail(r.ID)
		if err == nil {
			out = append(out, *d)
		}
	}
	return out, nil
}

// GetRuleDetail 返回规则详情。
func (s *Service) GetRuleDetail(id string) (*RuleDetail, error) {
	return s.ruleDetail(id)
}

// CheckRuleCycle 对当前规则集执行循环检测。
func (s *Service) CheckRuleCycle() error {
	ruleFontsAll, _ := s.allRuleFonts()
	return fallback.CheckCycle(ruleFontsAll)
}

// ruleDetail 组装规则详情。
func (s *Service) ruleDetail(id string) (*RuleDetail, error) {
	r, err := s.st.GetRule(id)
	if err != nil {
		return nil, err
	}
	ruleFonts, _ := s.st.RuleFonts(id)
	fontIDs := make([]string, 0, len(ruleFonts))
	fontNames := make([]string, 0, len(ruleFonts))
	for _, rf := range ruleFonts {
		fontIDs = append(fontIDs, rf.FontID)
		if f, err := s.st.GetFont(rf.FontID); err == nil {
			fontNames = append(fontNames, f.Name)
		}
	}
	return &RuleDetail{Rule: *r, FontIDs: fontIDs, FontNames: fontNames, Checksum: r.Checksum}, nil
}

// ruleSnapshot 组装规则验证快照。
func (s *Service) ruleSnapshot(id string) (fallback.RuleSnapshot, error) {
	r, err := s.st.GetRule(id)
	if err != nil {
		return fallback.RuleSnapshot{}, err
	}
	ruleFonts, _ := s.st.RuleFonts(id)
	fontIDs := make([]string, 0, len(ruleFonts))
	fonts := make([]model.Font, 0, len(ruleFonts))
	rangesByFont := map[string][]model.Range{}
	for _, rf := range ruleFonts {
		fontIDs = append(fontIDs, rf.FontID)
		f, err := s.st.GetFont(rf.FontID)
		if err != nil {
			continue
		}
		fonts = append(fonts, *f)
		rs, _ := s.st.FontRanges(rf.FontID)
		var norm []model.Range
		for _, rr := range rs {
			norm = append(norm, model.Range{Start: rr.Start, End: rr.End})
		}
		rangesByFont[rf.FontID] = norm
	}
	return fallback.RuleSnapshot{Rule: *r, FontIDs: fontIDs, Fonts: fonts, RangesByFont: rangesByFont}, nil
}

// allRuleFonts 返回全部规则字体绑定（ruleID → fontIDs 有序）。
func (s *Service) allRuleFonts() (map[string][]string, error) {
	rules, err := s.st.ListRules()
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, r := range rules {
		ruleFonts, _ := s.st.RuleFonts(r.ID)
		ids := make([]string, 0, len(ruleFonts))
		for _, rf := range ruleFonts {
			ids = append(ids, rf.FontID)
		}
		out[r.ID] = ids
	}
	return out, nil
}

// coverageView 构建覆盖视图。
func (s *Service) coverageView() *typeface.CoverageView {
	fonts, _ := s.st.ListFonts()
	ranges, _ := s.st.AllFontRanges()
	return typeface.NewCoverageView(fonts, ranges)
}

// joinReasons 将验证原因列表拼接为字符串。
func joinReasons(reasons []string) string {
	out := ""
	for i, r := range reasons {
		if i > 0 {
			out += "; "
		}
		out += r
	}
	return out
}
