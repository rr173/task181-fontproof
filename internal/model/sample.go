package model

import "time"

// 分析运行状态：running → completed / failed；重启时 running 恢复为 resumed 继续。
const (
	AnalysisRunning    = "running"
	AnalysisResumed    = "resumed"
	AnalysisCompleted  = "completed"
	AnalysisFailed     = "failed"
)

// 字素簇状态：已覆盖 / 需回退 / 拆分风险 / 缺字（终端状态，分析时计算）。
const (
	ClusterCovered  = "covered"
	ClusterFallback = "fallback"
	ClusterRisk     = "risk"
	ClusterMissing  = "missing"
)

// SampleSet 是待分析的文本样本集。
type SampleSet struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Analysis 是一次字素簇覆盖分析运行。
type Analysis struct {
	ID          string    `json:"id"`
	SampleSetID string    `json:"sample_set_id"`
	Status      string    `json:"status"`
	RuleVersion int       `json:"rule_version"`
	Stats       string    `json:"stats"` // JSON: {"total","covered","fallback","risk","missing"}
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Grapheme 是分析产出的一个字素簇及其证明结果。
type Grapheme struct {
	ID         string   `json:"id"`
	AnalysisID string   `json:"analysis_id"`
	Position   int      `json:"position"`
	Text       string   `json:"text"`
	Codepoints []rune   `json:"codepoints"`
	Script     string   `json:"script"`
	Status     string   `json:"status"`
	Chain      []string `json:"chain"`       // 字体选择链（font id 序列）
	RiskReason string   `json:"risk_reason"` // 拆分风险或缺字原因
	IsComposite bool    `json:"is_composite"` // 是否复合簇（组合标记/变体/ZWJ/数字序列）
}

// AnalysisStats 是分析统计的强类型形态。
type AnalysisStats struct {
	Total    int `json:"total"`
	Covered  int `json:"covered"`
	Fallback int `json:"fallback"`
	Risk     int `json:"risk"`
	Missing  int `json:"missing"`
}
