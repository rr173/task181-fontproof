package model

import "time"

// 覆盖规则状态机：draft → verifiable / gapped → published → superseded。
// 已发布（published）规则不可直接编辑，只能 supersede 生成新规则。
const (
	RuleDraft       = "draft"
	RuleVerifiable  = "verifiable"
	RuleGapped      = "gapped"
	RulePublished   = "published"
	RuleSuperseded  = "superseded"
)

// FallbackRule 是一条字体回退规则：声明必需脚本与按优先级排序的字体集合。
type FallbackRule struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Priority        int       `json:"priority"`
	Status          string    `json:"status"`
	RequiredScripts []string  `json:"required_scripts"`
	Checksum        string    `json:"checksum"`
	Version         int       `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	SupersededBy    string    `json:"superseded_by,omitempty"`
	Supersedes      string    `json:"supersedes,omitempty"`
}

// RuleFont 是规则与字体的绑定（rank 决定回退顺序，越小越优先）。
type RuleFont struct {
	RuleID string `json:"rule_id"`
	FontID string `json:"font_id"`
	Rank   int    `json:"rank"`
}

// RuleInput 是创建/编辑规则的请求载荷。
type RuleInput struct {
	Name            string   `json:"name"`
	Priority        int      `json:"priority"`
	RequiredScripts []string `json:"required_scripts"`
	FontIDs         []string `json:"font_ids"`
}

// RuleSetChecksum 计算一组规则（按 priority 排序）的指纹，用于重排冲突检测。
type RuleSetChecksum struct {
	Rules []string `json:"rules"` // "priority|ruleID|fontIDs"
	Hash  string   `json:"hash"`
}
