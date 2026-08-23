package model

import "time"

// 字体资产状态机：pending_scan → available / conflict → disabled；
// available/conflict 可重新扫描回到 available/conflict；disabled 可 enable 回 available。
const (
	FontPendingScan = "pending_scan"
	FontAvailable   = "available"
	FontConflict    = "conflict"
	FontDisabled    = "disabled"
)

// Font 表示一份字体资产及其覆盖摘要。
type Font struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Family          string    `json:"family"`
	Status          string    `json:"status"`
	Fingerprint     string    `json:"fingerprint"`
	SpecVersion     string    `json:"spec_version"`
	TotalCodepoints int       `json:"total_codepoints"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// FontRange 是字体覆盖的连续 Unicode 码点区间。
type FontRange struct {
	ID     string `json:"id"`
	FontID string `json:"font_id"`
	Start  rune   `json:"start"`
	End    rune   `json:"end"`
}

// FontFeature 是字体声明的 OpenType 特性标签（ccmp/mark/locl/…）。
type FontFeature struct {
	ID     string `json:"id"`
	FontID string `json:"font_id"`
	Tag    string `json:"tag"`
}

// FontScript 是字体覆盖的 Unicode 脚本标签（Latn/Arab/Deva/…）。
type FontScript struct {
	ID     string `json:"id"`
	FontID string `json:"font_id"`
	Script string `json:"script"`
}

// FontInput 是登记字体时的请求载荷。
type FontInput struct {
	Name        string   `json:"name"`
	Family      string   `json:"family"`
	SpecVersion string   `json:"spec_version"`
	Ranges      []Range  `json:"ranges"`
	Features    []string `json:"features"`
	Scripts     []string `json:"scripts"`
	Notes       string   `json:"notes"`
}

// Range 是 JSON 形式的码点区间。
type Range struct {
	Start rune `json:"start"`
	End   rune `json:"end"`
}
