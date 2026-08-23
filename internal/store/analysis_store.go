package store

import (
	"database/sql"
	"encoding/json"

	"task181-fontproof/internal/model"
)

// CreateSampleSet 创建样本集。
func (s *Store) CreateSampleSet(ss model.SampleSet) error {
	_, err := s.db.Exec(`INSERT INTO sample_sets (id, name, content, created_at) VALUES (?, ?, ?, ?)`,
		ss.ID, ss.Name, ss.Content, ss.CreatedAt.Format(timeFmt))
	return err
}

// GetSampleSet 按 id 获取样本集。
func (s *Store) GetSampleSet(id string) (*model.SampleSet, error) {
	var ss model.SampleSet
	var createdAt string
	err := s.db.QueryRow(`SELECT id, name, content, created_at FROM sample_sets WHERE id = ?`, id).
		Scan(&ss.ID, &ss.Name, &ss.Content, &createdAt)
	if err == sql.ErrNoRows {
		return nil, model.ENotFound("sample set")
	}
	if err != nil {
		return nil, err
	}
	ss.CreatedAt = parseTime(createdAt)
	return &ss, nil
}

// ListSampleSets 返回全部样本集（新→旧）。
func (s *Store) ListSampleSets() ([]model.SampleSet, error) {
	rows, err := s.db.Query(`SELECT id, name, content, created_at FROM sample_sets ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.SampleSet
	for rows.Next() {
		var ss model.SampleSet
		var createdAt string
		if err := rows.Scan(&ss.ID, &ss.Name, &ss.Content, &createdAt); err != nil {
			return nil, err
		}
		ss.CreatedAt = parseTime(createdAt)
		out = append(out, ss)
	}
	return out, rows.Err()
}

// CreateAnalysis 创建分析运行（status=running）。
func (s *Store) CreateAnalysis(a model.Analysis) error {
	_, err := s.db.Exec(`INSERT INTO analyses (id, sample_set_id, status, rule_version, stats, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.SampleSetID, a.Status, a.RuleVersion, a.Stats, a.CreatedAt.Format(timeFmt), a.UpdatedAt.Format(timeFmt))
	return err
}

// GetAnalysis 按 id 获取分析。
func (s *Store) GetAnalysis(id string) (*model.Analysis, error) {
	var a model.Analysis
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id, sample_set_id, status, rule_version, stats, created_at, updated_at FROM analyses WHERE id = ?`, id).
		Scan(&a.ID, &a.SampleSetID, &a.Status, &a.RuleVersion, &a.Stats, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, model.ENotFound("analysis")
	}
	if err != nil {
		return nil, err
	}
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	return &a, nil
}

// ListAnalysesBySample 返回样本集的分析列表。
func (s *Store) ListAnalysesBySample(sampleID string) ([]model.Analysis, error) {
	rows, err := s.db.Query(`SELECT id, sample_set_id, status, rule_version, stats, created_at, updated_at FROM analyses WHERE sample_set_id = ? ORDER BY created_at DESC`, sampleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Analysis
	for rows.Next() {
		var a model.Analysis
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.SampleSetID, &a.Status, &a.RuleVersion, &a.Stats, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTime(createdAt)
		a.UpdatedAt = parseTime(updatedAt)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListAllAnalyses 返回全部分析（新→旧），供启动恢复扫描未完成任务。
func (s *Store) ListAllAnalyses() ([]model.Analysis, error) {
	rows, err := s.db.Query(`SELECT id, sample_set_id, status, rule_version, stats, created_at, updated_at FROM analyses ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Analysis
	for rows.Next() {
		var a model.Analysis
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.SampleSetID, &a.Status, &a.RuleVersion, &a.Stats, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTime(createdAt)
		a.UpdatedAt = parseTime(updatedAt)
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAnalysisStatus 更新分析状态（running → completed/failed/resumed）。
func (s *Store) UpdateAnalysisStatus(id, status, stats string) error {
	res, err := s.db.Exec(`UPDATE analyses SET status = ?, stats = ?, updated_at = ? WHERE id = ?`,
		status, stats, nowISO(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ENotFound("analysis")
	}
	return nil
}

// CountGraphemes 返回分析已写入的字素簇数量（断点续传用）。
func (s *Store) CountGraphemes(analysisID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM graphemes WHERE analysis_id = ?`, analysisID).Scan(&n)
	return n, err
}

// CreateGraphemes 批量写入字素簇结果。
func (s *Store) CreateGraphemes(gs []model.Grapheme) error {
	return s.Tx(func(tx *sql.Tx) error {
		for _, g := range gs {
			composite := 0
			if g.IsComposite {
				composite = 1
			}
			if _, err := tx.Exec(`INSERT INTO graphemes (id, analysis_id, position, text, codepoints, script, status, chain, risk_reason, is_composite) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				g.ID, g.AnalysisID, g.Position, g.Text, MarshalCodepoints(g.Codepoints), g.Script, g.Status, MarshalChain(g.Chain), g.RiskReason, composite); err != nil {
				return err
			}
		}
		return nil
	})
}

// ListGraphemes 返回分析的全部字素簇（按 position 排序）。
func (s *Store) ListGraphemes(analysisID string) ([]model.Grapheme, error) {
	rows, err := s.db.Query(`SELECT id, analysis_id, position, text, codepoints, script, status, chain, risk_reason, is_composite FROM graphemes WHERE analysis_id = ? ORDER BY position`, analysisID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Grapheme
	for rows.Next() {
		var g model.Grapheme
		var cps, chain string
		var composite int
		if err := rows.Scan(&g.ID, &g.AnalysisID, &g.Position, &g.Text, &cps, &g.Script, &g.Status, &chain, &g.RiskReason, &composite); err != nil {
			return nil, err
		}
		g.Codepoints = UnmarshalCodepoints(cps)
		g.Chain = UnmarshalChain(chain)
		g.IsComposite = composite == 1
		out = append(out, g)
	}
	return out, rows.Err()
}

// StatsJSON 将统计序列化为 JSON。
func StatsJSON(stats model.AnalysisStats) string {
	b, _ := json.Marshal(stats)
	return string(b)
}

// UnmarshalStats 反序列化统计。
func UnmarshalStats(data string) model.AnalysisStats {
	var s model.AnalysisStats
	_ = json.Unmarshal([]byte(data), &s)
	return s
}
