// Package store 提供 SQLite 持久化层：schema 迁移、字体/规则/分析/报告/配置的 CRUD。
//
// 使用纯 Go 驱动 modernc.org/sqlite（CGO 无关）。所有跨实体写入经事务执行；
// 分析运行断点续传语义：running 状态的分析在重启后恢复为 resumed 并继续。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"task181-fontproof/internal/model"
)

// Store 封装 SQLite 连接与 prepared statement 的公共工厂。
type Store struct {
	db *sql.DB
}

// Open 打开（或创建）SQLite 数据库并执行迁移。
func Open(path string) (*Store, error) {
	if path != "" && path != ":memory:" {
		if dir := filepath.Dir(path); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create db dir: %w", err)
			}
		}
	}
	dsn := path
	if dsn == "" {
		dsn = ":memory:"
	} else if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写者
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error {
	return s.db.Close()
}

// DB 暴露底层连接（供事务与测试）。
func (s *Store) DB() *sql.DB { return s.db }

// migrate 执行 schema 迁移（幂等，CREATE TABLE IF NOT EXISTS）。
func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS fonts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			family TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			fingerprint TEXT NOT NULL UNIQUE,
			spec_version TEXT NOT NULL DEFAULT '',
			total_codepoints INTEGER NOT NULL DEFAULT 0,
			notes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS font_ranges (
			id TEXT PRIMARY KEY,
			font_id TEXT NOT NULL REFERENCES fonts(id) ON DELETE CASCADE,
			start_cp INTEGER NOT NULL,
			end_cp INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_font_ranges_font ON font_ranges(font_id)`,
		`CREATE TABLE IF NOT EXISTS font_features (
			id TEXT PRIMARY KEY,
			font_id TEXT NOT NULL REFERENCES fonts(id) ON DELETE CASCADE,
			tag TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_font_features_font ON font_features(font_id)`,
		`CREATE TABLE IF NOT EXISTS font_scripts (
			id TEXT PRIMARY KEY,
			font_id TEXT NOT NULL REFERENCES fonts(id) ON DELETE CASCADE,
			script TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_font_scripts_font ON font_scripts(font_id)`,
		`CREATE TABLE IF NOT EXISTS fallback_rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			priority INTEGER NOT NULL,
			status TEXT NOT NULL,
			required_scripts TEXT NOT NULL DEFAULT '[]',
			checksum TEXT NOT NULL DEFAULT '',
			version INTEGER NOT NULL DEFAULT 1,
			superseded_by TEXT NOT NULL DEFAULT '',
			supersedes TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rule_fonts (
			rule_id TEXT NOT NULL REFERENCES fallback_rules(id) ON DELETE CASCADE,
			font_id TEXT NOT NULL REFERENCES fonts(id) ON DELETE CASCADE,
			rank INTEGER NOT NULL,
			PRIMARY KEY (rule_id, font_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rule_fonts_font ON rule_fonts(font_id)`,
		`CREATE TABLE IF NOT EXISTS sample_sets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS analyses (
			id TEXT PRIMARY KEY,
			sample_set_id TEXT NOT NULL REFERENCES sample_sets(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			rule_version INTEGER NOT NULL DEFAULT 0,
			stats TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS graphemes (
			id TEXT PRIMARY KEY,
			analysis_id TEXT NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
			position INTEGER NOT NULL,
			text TEXT NOT NULL,
			codepoints TEXT NOT NULL,
			script TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			chain TEXT NOT NULL DEFAULT '[]',
			risk_reason TEXT NOT NULL DEFAULT '',
			is_composite INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_graphemes_analysis ON graphemes(analysis_id, position)`,
		`CREATE TABLE IF NOT EXISTS reports (
			id TEXT PRIMARY KEY,
			analysis_id TEXT NOT NULL REFERENCES analyses(id) ON DELETE CASCADE,
			rule_version INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			stats TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			published_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS published_configs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			rule_version INTEGER NOT NULL,
			checksum TEXT NOT NULL,
			status TEXT NOT NULL,
			snapshot TEXT NOT NULL,
			created_at TEXT NOT NULL,
			superseded_at TEXT NOT NULL DEFAULT '',
			superseded_by TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS config_versions (
			id TEXT PRIMARY KEY,
			config_id TEXT NOT NULL REFERENCES published_configs(id) ON DELETE CASCADE,
			version INTEGER NOT NULL,
			checksum TEXT NOT NULL,
			snapshot TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// Tx 执行带事务的回调；回调返回 error 时回滚。
func (s *Store) Tx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Audit 记录审计事件。
func (s *Store) Audit(actor, action, targetType, targetID, detail string) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_events (id, actor, action, target_type, target_id, detail, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		newID("aud"), actor, action, targetType, targetID, detail, nowISO(),
	)
	return err
}

// ListAudit 返回审计事件（新→旧）。
func (s *Store) ListAudit(limit int) ([]model.AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, actor, action, target_type, target_id, detail, created_at FROM audit_events ORDER BY created_at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.AuditEvent
	for rows.Next() {
		var e model.AuditEvent
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.TargetType, &e.TargetID, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
