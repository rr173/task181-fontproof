package service

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
