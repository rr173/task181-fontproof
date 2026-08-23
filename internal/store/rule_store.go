package store

import (
	"database/sql"
	"encoding/json"

	"task181-fontproof/internal/model"
)

// CreateRule 创建回退规则（事务：主记录 + 字体绑定）。
func (s *Store) CreateRule(r model.FallbackRule, fonts []model.RuleFont) error {
	return s.Tx(func(tx *sql.Tx) error {
		scripts, _ := json.Marshal(r.RequiredScripts)
		if _, err := tx.Exec(`INSERT INTO fallback_rules (id, name, priority, status, required_scripts, checksum, version, superseded_by, supersedes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, r.Name, r.Priority, r.Status, string(scripts), r.Checksum, r.Version, r.SupersededBy, r.Supersedes, r.CreatedAt.Format(timeFmt), r.UpdatedAt.Format(timeFmt)); err != nil {
			return err
		}
		for _, rf := range fonts {
			if _, err := tx.Exec(`INSERT INTO rule_fonts (rule_id, font_id, rank) VALUES (?, ?, ?)`, rf.RuleID, rf.FontID, rf.Rank); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetRule 按 id 获取规则。
func (s *Store) GetRule(id string) (*model.FallbackRule, error) {
	var r model.FallbackRule
	var scripts, createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id, name, priority, status, required_scripts, checksum, version, superseded_by, supersedes, created_at, updated_at FROM fallback_rules WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.Priority, &r.Status, &scripts, &r.Checksum, &r.Version, &r.SupersededBy, &r.Supersedes, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, model.ENotFound("rule")
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(scripts), &r.RequiredScripts)
	r.CreatedAt = parseTime(createdAt)
	r.UpdatedAt = parseTime(updatedAt)
	return &r, nil
}

// ListRules 返回全部规则（按 priority 升序）。
func (s *Store) ListRules() ([]model.FallbackRule, error) {
	rows, err := s.db.Query(`SELECT id, name, priority, status, required_scripts, checksum, version, superseded_by, supersedes, created_at, updated_at FROM fallback_rules ORDER BY priority, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FallbackRule
	for rows.Next() {
		var r model.FallbackRule
		var scripts, createdAt, updatedAt string
		if err := rows.Scan(&r.ID, &r.Name, &r.Priority, &r.Status, &scripts, &r.Checksum, &r.Version, &r.SupersededBy, &r.Supersedes, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(scripts), &r.RequiredScripts)
		r.CreatedAt = parseTime(createdAt)
		r.UpdatedAt = parseTime(updatedAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

// RuleFonts 返回规则绑定的字体（按 rank 升序）。
func (s *Store) RuleFonts(ruleID string) ([]model.RuleFont, error) {
	rows, err := s.db.Query(`SELECT rule_id, font_id, rank FROM rule_fonts WHERE rule_id = ? ORDER BY rank`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.RuleFont
	for rows.Next() {
		var rf model.RuleFont
		if err := rows.Scan(&rf.RuleID, &rf.FontID, &rf.Rank); err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, rows.Err()
}

// AllRuleFonts 返回全部规则字体绑定（按 rule_id, rank）。
func (s *Store) AllRuleFonts() ([]model.RuleFont, error) {
	rows, err := s.db.Query(`SELECT rule_id, font_id, rank FROM rule_fonts ORDER BY rule_id, rank`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.RuleFont
	for rows.Next() {
		var rf model.RuleFont
		if err := rows.Scan(&rf.RuleID, &rf.FontID, &rf.Rank); err != nil {
			return nil, err
		}
		out = append(out, rf)
	}
	return out, rows.Err()
}

// UpdateRule 更新规则主记录（名称/优先级/必需脚本/校验和/版本，乐观锁）。
// expectedVersion < 0 表示忽略版本校验。
func (s *Store) UpdateRule(r model.FallbackRule, expectedVersion int) error {
	scripts, _ := json.Marshal(r.RequiredScripts)
	res, err := s.db.Exec(`UPDATE fallback_rules SET name = ?, priority = ?, status = ?, required_scripts = ?, checksum = ?, version = ?, updated_at = ? WHERE id = ? AND (version = ? OR ? < 0)`,
		r.Name, r.Priority, r.Status, string(scripts), r.Checksum, r.Version, r.UpdatedAt.Format(timeFmt), r.ID, expectedVersion, expectedVersion)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// 区分不存在与版本冲突
		if _, gerr := s.GetRule(r.ID); gerr != nil {
			return gerr
		}
		return model.EConflict("rule version conflict: another engineer updated the rule")
	}
	return nil
}

// UpdateRuleStatus 更新规则状态（供 validate/publish/supersede）。
func (s *Store) UpdateRuleStatus(id, status string, version int) error {
	res, err := s.db.Exec(`UPDATE fallback_rules SET status = ?, version = ?, updated_at = ? WHERE id = ?`,
		status, version, nowISO(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ENotFound("rule")
	}
	return nil
}

// UpdateRulePriority 仅更新规则优先级（供重排写回，不校验版本）。
func (s *Store) UpdateRulePriority(id string, priority int) error {
	res, err := s.db.Exec(`UPDATE fallback_rules SET priority = ?, updated_at = ? WHERE id = ?`,
		priority, nowISO(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ENotFound("rule")
	}
	return nil
}

// SetRuleSupersession 记录规则的替代关系（published → superseded）。
func (s *Store) SetRuleSupersession(id, supersededBy string) error {
	res, err := s.db.Exec(`UPDATE fallback_rules SET status = ?, superseded_by = ?, version = version + 1, updated_at = ? WHERE id = ?`,
		model.RuleSuperseded, supersededBy, nowISO(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ENotFound("rule")
	}
	return nil
}

// SetRuleFonts 事务性重设规则字体绑定（绑定/解绑）。
func (s *Store) SetRuleFonts(ruleID string, fonts []model.RuleFont) error {
	return s.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM rule_fonts WHERE rule_id = ?`, ruleID); err != nil {
			return err
		}
		for _, rf := range fonts {
			if _, err := tx.Exec(`INSERT INTO rule_fonts (rule_id, font_id, rank) VALUES (?, ?, ?)`, rf.RuleID, rf.FontID, rf.Rank); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteRuleFont 解绑规则下的单个字体。
func (s *Store) DeleteRuleFont(ruleID, fontID string) error {
	res, err := s.db.Exec(`DELETE FROM rule_fonts WHERE rule_id = ? AND font_id = ?`, ruleID, fontID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ENotFound("rule-font binding")
	}
	return nil
}

// CurrentRuleVersion 返回当前规则集版本（最新规则 version 的 max，或 0）。
func (s *Store) CurrentRuleVersion() (int, error) {
	var v sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(version) FROM fallback_rules`).Scan(&v); err != nil {
		return 0, err
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}
