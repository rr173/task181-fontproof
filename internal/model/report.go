package model

import "time"

// 覆盖报告状态机：generating → pending_review → passed / gapped → published。
const (
	ReportGenerating   = "generating"
	ReportPendingReview = "pending_review"
	ReportPassed       = "passed"
	ReportGapped       = "gapped"
	ReportPublished    = "published"
)

// 已发布配置状态：active → superseded。
const (
	ConfigActive     = "active"
	ConfigSuperseded = "superseded"
)

// CoverageReport 是一次覆盖证明的报告。
type CoverageReport struct {
	ID          string    `json:"id"`
	AnalysisID  string    `json:"analysis_id"`
	RuleVersion int       `json:"rule_version"`
	Status      string    `json:"status"`
	Stats       string    `json:"stats"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PublishedAt time.Time `json:"published_at,omitempty"`
}

// PublishedConfig 是冻结的回退配置快照（发布后不可变，只能被替代）。
type PublishedConfig struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RuleVersion int       `json:"rule_version"`
	Checksum    string    `json:"checksum"`
	Status      string    `json:"status"`
	Snapshot    string    `json:"snapshot"` // JSON: {"rules":[...],"fonts":[...]}
	CreatedAt   time.Time `json:"created_at"`
	SupersededAt time.Time `json:"superseded_at,omitempty"`
	SupersededBy string    `json:"superseded_by,omitempty"`
}

// ConfigVersion 是配置的版本化历史记录，供版本比较使用。
type ConfigVersion struct {
	ID         string    `json:"id"`
	ConfigID   string    `json:"config_id"`
	Version    int       `json:"version"`
	Checksum   string    `json:"checksum"`
	Snapshot   string    `json:"snapshot"`
	CreatedAt  time.Time `json:"created_at"`
}

// ConfigDiff 是一次版本比较的输出。
type ConfigDiff struct {
	BaseID        string   `json:"base_id"`
	TargetID      string   `json:"target_id"`
	AddedRules    []string `json:"added_rules"`
	RemovedRules  []string `json:"removed_rules"`
	Reordered     bool     `json:"reordered"`
	ChangedFonts  []string `json:"changed_fonts"`
	Same          bool     `json:"same"`
	Summary       string   `json:"summary"`
}
