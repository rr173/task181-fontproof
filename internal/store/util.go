package store

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// newID 生成唯一标识符（前缀 + 随机字节）。
func newID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return prefix + "-" + hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return prefix + "-" + hex.EncodeToString(b)
}

// nowISO 返回当前时间的 ISO8601 字符串。
func nowISO() string {
	return time.Now().UTC().Format(timeFmt)
}

// parseTime 解析 ISO8601 时间字符串。
func parseTime(s string) time.Time {
	t, _ := time.Parse(timeFmt, s)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02T15:04:05Z07:00", s)
	}
	return t
}
