package service

import (
	"time"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/typeface"
)

// FontResult 是一次登记/扫描的结果。
type FontResult struct {
	Font     model.Font  `json:"font"`
	Reused   bool        `json:"reused"` // 指纹幂等复用
	Ranges   []model.FontRange `json:"ranges"`
	Features []string    `json:"features"`
	Scripts  []string    `json:"scripts"`
}

// RegisterFont 登记字体资产（幂等：相同指纹复用既有扫描结果）。
// 登记后状态为 pending_scan，等待 scan 触发实际评估。
func (s *Service) RegisterFont(in model.FontInput) (*FontResult, error) {
	fp := typeface.Fingerprint(in)
	if existing, err := s.st.FindFontByFingerprint(fp); err == nil {
		ranges, _ := s.st.FontRanges(existing.ID)
		features, _ := s.st.FontFeatures(existing.ID)
		scripts, _ := s.st.FontScripts(existing.ID)
		return &FontResult{Font: *existing, Reused: true, Ranges: ranges, Features: features, Scripts: scripts}, nil
	}
	normalized := typeface.NormalizeRanges(in.Ranges)
	if len(normalized) == 0 {
		return nil, model.E("bad_request", model.ErrEmptyCoverage, "font must declare at least one coverage range")
	}
	f := model.Font{
		ID:              newEntityID("fnt"),
		Name:            in.Name,
		Family:          in.Family,
		Status:          model.FontPendingScan,
		Fingerprint:     fp,
		SpecVersion:     in.SpecVersion,
		TotalCodepoints: typeface.CountCodepoints(normalized),
		Notes:           in.Notes,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	var ranges []model.FontRange
	for _, r := range normalized {
		ranges = append(ranges, model.FontRange{ID: newEntityID("frt"), FontID: f.ID, Start: r.Start, End: r.End})
	}
	var features []model.FontFeature
	for _, tag := range uniqueStrings(in.Features) {
		features = append(features, model.FontFeature{ID: newEntityID("ffe"), FontID: f.ID, Tag: tag})
	}
	var scripts []model.FontScript
	for _, sc := range uniqueStrings(in.Scripts) {
		scripts = append(scripts, model.FontScript{ID: newEntityID("fsc"), FontID: f.ID, Script: sc})
	}
	if err := s.st.CreateFont(f, ranges, features, scripts); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "register_font", "font", f.ID, f.Name)
	return &FontResult{Font: f, Ranges: ranges, Features: in.Features, Scripts: in.Scripts}, nil
}

// ScanFont 对字体资产执行扫描：评估覆盖冲突并推进状态机。
func (s *Service) ScanFont(id string) (*model.Font, error) {
	f, err := s.st.GetFont(id)
	if err != nil {
		return nil, err
	}
	if f.Status != model.FontPendingScan && f.Status != model.FontAvailable && f.Status != model.FontConflict {
		return nil, model.EInvalidState("only pending/available/conflict fonts can be scanned")
	}
	ranges, _ := s.st.FontRanges(id)
	normalized := make([]model.Range, 0, len(ranges))
	for _, r := range ranges {
		normalized = append(normalized, model.Range{Start: r.Start, End: r.End})
	}
	// 与其它 active 字体检测覆盖冲突
	conflict := false
	others, _ := s.st.ListFonts()
	for _, o := range others {
		if o.ID == id || o.Status == model.FontDisabled || o.Status == model.FontPendingScan {
			continue
		}
		oRanges, _ := s.st.FontRanges(o.ID)
		oN := make([]model.Range, 0, len(oRanges))
		for _, r := range oRanges {
			oN = append(oN, model.Range{Start: r.Start, End: r.End})
		}
		if typeface.DetectCoverageConflict(normalized, oN) {
			conflict = true
			break
		}
	}
	outcome := typeface.EvaluateScan(typeface.CountCodepoints(normalized), conflict)
	if err := s.st.UpdateFontStatus(id, outcome.Status); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "scan_font", "font", id, outcome.Reason)
	return s.st.GetFont(id)
}

// DisableFont 停用字体资产（available/conflict → disabled）。
func (s *Service) DisableFont(id string) (*model.Font, error) {
	f, err := s.st.GetFont(id)
	if err != nil {
		return nil, err
	}
	if !typeface.CanTransition(f.Status, model.FontDisabled) {
		return nil, model.EInvalidState("font cannot transition to disabled from " + f.Status)
	}
	if err := s.st.UpdateFontStatus(id, model.FontDisabled); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "disable_font", "font", id, "")
	return s.st.GetFont(id)
}

// EnableFont 重新启用字体资产（disabled → available）。
func (s *Service) EnableFont(id string) (*model.Font, error) {
	f, err := s.st.GetFont(id)
	if err != nil {
		return nil, err
	}
	if !typeface.CanTransition(f.Status, model.FontAvailable) {
		return nil, model.EInvalidState("font cannot transition to available from " + f.Status)
	}
	if err := s.st.UpdateFontStatus(id, model.FontAvailable); err != nil {
		return nil, err
	}
	s.addAudit("engineer", "enable_font", "font", id, "")
	return s.st.GetFont(id)
}

// ListFonts 返回字体列表（含区间、特性、脚本）。
func (s *Service) ListFonts() ([]model.Font, error) {
	return s.st.ListFonts()
}

// GetFontDetail 返回字体详情（含区间、特性、脚本）。
func (s *Service) GetFontDetail(id string) (*FontResult, error) {
	f, err := s.st.GetFont(id)
	if err != nil {
		return nil, err
	}
	ranges, _ := s.st.FontRanges(id)
	features, _ := s.st.FontFeatures(id)
	scripts, _ := s.st.FontScripts(id)
	return &FontResult{Font: *f, Ranges: ranges, Features: features, Scripts: scripts}, nil
}

func newEntityID(prefix string) string {
	return newID(prefix)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
