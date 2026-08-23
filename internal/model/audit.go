package model

import "time"

// AuditEvent 记录关键业务动作，用于追溯发布、停用与替代。
type AuditEvent struct {
	ID         string    `json:"id"`
	Actor      string    `json:"actor"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}
