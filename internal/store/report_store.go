package store

import (
	"database/sql"
	"encoding/json"

	"task181-fontproof/internal/model"
)

// CreateReport 创建覆盖报告（status=generating）。
func (s *Store) CreateReport(r model.CoverageReport) error {
	_, err := s.db.Exec(`INSERT INTO reports (id, analysis_id, rule_version, status, stats, created_at, updated_at, published_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.AnalysisID, r.RuleVersion, r.Status, r.Stats, r.CreatedAt.Format(timeFmt), r.UpdatedAt.Format(timeFmt), r.PublishedAt.Format(timeFmt))
	return err
}

// GetReport 按 id 获取报告。
func (s *Store) GetReport(id string) (*model.CoverageReport, error) {
	var r model.CoverageReport
	var createdAt, updatedAt, publishedAt string
	err := s.db.QueryRow(`SELECT id, analysis_id, rule_version, status, stats, created_at, updated_at, published_at FROM reports WHERE id = ?`, id).
		Scan(&r.ID, &r.AnalysisID, &r.RuleVersion, &r.Status, &r.Stats, &createdAt, &updatedAt, &publishedAt)
	if err == sql.ErrNoRows {
		return nil, model.ENotFound("report")
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt = parseTime(createdAt)
	r.UpdatedAt = parseTime(updatedAt)
	r.PublishedAt = parseTime(publishedAt)
	return &r, nil
}

// ListReports 返回全部报告（新→旧）。
func (s *Store) ListReports() ([]model.CoverageReport, error) {
	rows, err := s.db.Query(`SELECT id, analysis_id, rule_version, status, stats, created_at, updated_at, published_at FROM reports ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.CoverageReport
	for rows.Next() {
		var r model.CoverageReport
		var createdAt, updatedAt, publishedAt string
		if err := rows.Scan(&r.ID, &r.AnalysisID, &r.RuleVersion, &r.Status, &r.Stats, &createdAt, &updatedAt, &publishedAt); err != nil {
			return nil, err
		}
		r.CreatedAt = parseTime(createdAt)
		r.UpdatedAt = parseTime(updatedAt)
		r.PublishedAt = parseTime(publishedAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateReportStatus 更新报告状态并可选写入发布时间。
func (s *Store) UpdateReportStatus(id, status, publishedAt string) error {
	res, err := s.db.Exec(`UPDATE reports SET status = ?, published_at = ?, updated_at = ? WHERE id = ?`,
		status, publishedAt, nowISO(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ENotFound("report")
	}
	return nil
}

// CreatePublishedConfig 创建已发布配置（含快照）并记录首个版本历史。
func (s *Store) CreatePublishedConfig(c model.PublishedConfig) error {
	return s.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO published_configs (id, name, rule_version, checksum, status, snapshot, created_at, superseded_at, superseded_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.ID, c.Name, c.RuleVersion, c.Checksum, c.Status, c.Snapshot, c.CreatedAt.Format(timeFmt), c.SupersededAt.Format(timeFmt), c.SupersededBy); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO config_versions (id, config_id, version, checksum, snapshot, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			c.ID, c.ID, 1, c.Checksum, c.Snapshot, c.CreatedAt.Format(timeFmt)); err != nil {
			return err
		}
		return nil
	})
}

// GetPublishedConfig 按 id 获取已发布配置。
func (s *Store) GetPublishedConfig(id string) (*model.PublishedConfig, error) {
	var c model.PublishedConfig
	var createdAt, supersededAt string
	err := s.db.QueryRow(`SELECT id, name, rule_version, checksum, status, snapshot, created_at, superseded_at, superseded_by FROM published_configs WHERE id = ?`, id).
		Scan(&c.ID, &c.Name, &c.RuleVersion, &c.Checksum, &c.Status, &c.Snapshot, &createdAt, &supersededAt, &c.SupersededBy)
	if err == sql.ErrNoRows {
		return nil, model.ENotFound("published config")
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt = parseTime(createdAt)
	c.SupersededAt = parseTime(supersededAt)
	return &c, nil
}

// ListPublishedConfigs 返回全部已发布配置（新→旧）。
func (s *Store) ListPublishedConfigs() ([]model.PublishedConfig, error) {
	rows, err := s.db.Query(`SELECT id, name, rule_version, checksum, status, snapshot, created_at, superseded_at, superseded_by FROM published_configs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PublishedConfig
	for rows.Next() {
		var c model.PublishedConfig
		var createdAt, supersededAt string
		if err := rows.Scan(&c.ID, &c.Name, &c.RuleVersion, &c.Checksum, &c.Status, &c.Snapshot, &createdAt, &supersededAt, &c.SupersededBy); err != nil {
			return nil, err
		}
		c.CreatedAt = parseTime(createdAt)
		c.SupersededAt = parseTime(supersededAt)
		out = append(out, c)
	}
	return out, rows.Err()
}

// SupersedePublishedConfig 将旧配置标记为已替代（冻结链：superseded_by）。
func (s *Store) SupersedePublishedConfig(id, by string) error {
	res, err := s.db.Exec(`UPDATE published_configs SET status = ?, superseded_at = ?, superseded_by = ? WHERE id = ?`,
		model.ConfigSuperseded, nowISO(), by, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ENotFound("published config")
	}
	return nil
}

// AppendConfigVersion 为配置追加版本历史。
func (s *Store) AppendConfigVersion(v model.ConfigVersion) error {
	_, err := s.db.Exec(`INSERT INTO config_versions (id, config_id, version, checksum, snapshot, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		v.ID, v.ConfigID, v.Version, v.Checksum, v.Snapshot, v.CreatedAt.Format(timeFmt))
	return err
}

// ListConfigVersions 返回配置版本历史（新→旧）。
func (s *Store) ListConfigVersions(configID string) ([]model.ConfigVersion, error) {
	rows, err := s.db.Query(`SELECT id, config_id, version, checksum, snapshot, created_at FROM config_versions WHERE config_id = ? ORDER BY version DESC`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ConfigVersion
	for rows.Next() {
		var v model.ConfigVersion
		var createdAt string
		if err := rows.Scan(&v.ID, &v.ConfigID, &v.Version, &v.Checksum, &v.Snapshot, &createdAt); err != nil {
			return nil, err
		}
		v.CreatedAt = parseTime(createdAt)
		out = append(out, v)
	}
	return out, rows.Err()
}

// MarshalStrings 序列化字符串数组（JSON）。
func MarshalStrings(ss []string) string {
	b, _ := json.Marshal(ss)
	return string(b)
}
