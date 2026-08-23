package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"task181-fontproof/internal/model"
)

// CreateFont 事务性登记字体：主记录 + 区间 + 特性 + 脚本。
// 返回 ErrDuplicate（指纹已存在）时调用方应复用既有字体。
func (s *Store) CreateFont(f model.Font, ranges []model.FontRange, features []model.FontFeature, scripts []model.FontScript) error {
	return s.Tx(func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM fonts WHERE fingerprint = ?`, f.Fingerprint).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			return model.E("duplicate", model.ErrDuplicate, "font with same fingerprint already exists")
		}
		if _, err := tx.Exec(`INSERT INTO fonts (id, name, family, status, fingerprint, spec_version, total_codepoints, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			f.ID, f.Name, f.Family, f.Status, f.Fingerprint, f.SpecVersion, f.TotalCodepoints, f.Notes, f.CreatedAt.Format(timeFmt), f.UpdatedAt.Format(timeFmt)); err != nil {
			return err
		}
		for _, r := range ranges {
			if _, err := tx.Exec(`INSERT INTO font_ranges (id, font_id, start_cp, end_cp) VALUES (?, ?, ?, ?)`, r.ID, r.FontID, r.Start, r.End); err != nil {
				return err
			}
		}
		for _, ft := range features {
			if _, err := tx.Exec(`INSERT INTO font_features (id, font_id, tag) VALUES (?, ?, ?)`, ft.ID, ft.FontID, ft.Tag); err != nil {
				return err
			}
		}
		for _, sc := range scripts {
			if _, err := tx.Exec(`INSERT INTO font_scripts (id, font_id, script) VALUES (?, ?, ?)`, sc.ID, sc.FontID, sc.Script); err != nil {
				return err
			}
		}
		return nil
	})
}

// FindFontByFingerprint 按指纹查找既有字体（幂等导入复用）。
func (s *Store) FindFontByFingerprint(fp string) (*model.Font, error) {
	var f model.Font
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id, name, family, status, fingerprint, spec_version, total_codepoints, notes, created_at, updated_at FROM fonts WHERE fingerprint = ?`, fp).
		Scan(&f.ID, &f.Name, &f.Family, &f.Status, &f.Fingerprint, &f.SpecVersion, &f.TotalCodepoints, &f.Notes, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, model.ENotFound("font")
	}
	if err != nil {
		return nil, err
	}
	f.CreatedAt = parseTime(createdAt)
	f.UpdatedAt = parseTime(updatedAt)
	return &f, nil
}

// GetFont 按 id 获取字体主记录。
func (s *Store) GetFont(id string) (*model.Font, error) {
	var f model.Font
	var createdAt, updatedAt string
	err := s.db.QueryRow(`SELECT id, name, family, status, fingerprint, spec_version, total_codepoints, notes, created_at, updated_at FROM fonts WHERE id = ?`, id).
		Scan(&f.ID, &f.Name, &f.Family, &f.Status, &f.Fingerprint, &f.SpecVersion, &f.TotalCodepoints, &f.Notes, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, model.ENotFound("font")
	}
	if err != nil {
		return nil, err
	}
	f.CreatedAt = parseTime(createdAt)
	f.UpdatedAt = parseTime(updatedAt)
	return &f, nil
}

// ListFonts 返回全部字体（按名称排序）。
func (s *Store) ListFonts() ([]model.Font, error) {
	rows, err := s.db.Query(`SELECT id, name, family, status, fingerprint, spec_version, total_codepoints, notes, created_at, updated_at FROM fonts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Font
	for rows.Next() {
		var f model.Font
		var createdAt, updatedAt string
		if err := rows.Scan(&f.ID, &f.Name, &f.Family, &f.Status, &f.Fingerprint, &f.SpecVersion, &f.TotalCodepoints, &f.Notes, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		f.CreatedAt = parseTime(createdAt)
		f.UpdatedAt = parseTime(updatedAt)
		out = append(out, f)
	}
	return out, rows.Err()
}

// UpdateFontStatus 更新字体状态与更新时间（乐观无锁，状态机由 service 校验）。
func (s *Store) UpdateFontStatus(id, status string) error {
	res, err := s.db.Exec(`UPDATE fonts SET status = ?, updated_at = ? WHERE id = ?`, status, nowISO(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ENotFound("font")
	}
	return nil
}

// FontRanges 返回字体覆盖区间。
func (s *Store) FontRanges(fontID string) ([]model.FontRange, error) {
	rows, err := s.db.Query(`SELECT id, font_id, start_cp, end_cp FROM font_ranges WHERE font_id = ? ORDER BY start_cp`, fontID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FontRange
	for rows.Next() {
		var r model.FontRange
		if err := rows.Scan(&r.ID, &r.FontID, &r.Start, &r.End); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AllFontRanges 返回全部字体区间（构建覆盖视图用）。
func (s *Store) AllFontRanges() ([]model.FontRange, error) {
	rows, err := s.db.Query(`SELECT id, font_id, start_cp, end_cp FROM font_ranges ORDER BY font_id, start_cp`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FontRange
	for rows.Next() {
		var r model.FontRange
		if err := rows.Scan(&r.ID, &r.FontID, &r.Start, &r.End); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FontFeatures 返回字体特性标签。
func (s *Store) FontFeatures(fontID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT tag FROM font_features WHERE font_id = ? ORDER BY tag`, fontID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// FontScripts 返回字体覆盖脚本。
func (s *Store) FontScripts(fontID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT script FROM font_scripts WHERE font_id = ? ORDER BY script`, fontID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sc string
		if err := rows.Scan(&sc); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// MarshalCodepoints 将码点切片序列化为 JSON 数组（graphemes.codepoints 列）。
func MarshalCodepoints(cps []rune) string {
	b, _ := json.Marshal(cps)
	return string(b)
}

// MarshalChain 将字体链序列化为 JSON 数组。
func MarshalChain(chain []string) string {
	b, _ := json.Marshal(chain)
	return string(b)
}

// UnmarshalCodepoints 反序列化码点。
func UnmarshalCodepoints(data string) []rune {
	var out []rune
	_ = json.Unmarshal([]byte(data), &out)
	return out
}

// UnmarshalChain 反序列化字体链。
func UnmarshalChain(data string) []string {
	var out []string
	_ = json.Unmarshal([]byte(data), &out)
	return out
}

// Helper 输出时间格式常量（避免重复字符串）。
var timeFmt = "2006-01-02T15:04:05Z07:00"

// 供其他文件使用的辅助函数（见 util.go 的 newID/nowISO）。
var _ = fmt.Sprintf
