// Package service 编排跨模块业务：字体登记/扫描、规则生命周期、
// 样本分析与恢复、报告与配置发布。所有业务不变量在此层强制。
package service

import (
	"encoding/json"

	"task181-fontproof/internal/model"
	"task181-fontproof/internal/store"
)

// Service 聚合 store 与领域逻辑。
type Service struct {
	st *store.Store
}

// New 构造服务。
func New(st *store.Store) *Service {
	return &Service{st: st}
}

// Store 暴露底层存储（供测试与 httpapi 使用）。
func (s *Service) Store() *store.Store { return s.st }

// Recover 启动恢复：把上次进程未完成的分析（running）标记为 resumed，
// 供下一次 ResolveAnalysis 继续消费。返回恢复的分析 id 列表。
func (s *Service) Recover() ([]string, error) {
	analyses, err := s.st.ListAllAnalyses()
	if err != nil {
		return nil, err
	}
	var recovered []string
	for _, a := range analyses {
		if a.Status == model.AnalysisRunning {
			if err := s.st.UpdateAnalysisStatus(a.ID, model.AnalysisResumed, a.Stats); err == nil {
				recovered = append(recovered, a.ID)
			}
		}
	}
	return recovered, nil
}

// RecoverAll 返回全部运行中（含 resumed）未完成的分析，供继续执行。
func (s *Service) RecoverAll() ([]model.Analysis, error) {
	analyses, err := s.st.ListAllAnalyses()
	if err != nil {
		return nil, err
	}
	var out []model.Analysis
	for _, a := range analyses {
		if a.Status == model.AnalysisRunning || a.Status == model.AnalysisResumed {
			out = append(out, a)
		}
	}
	return out, nil
}

// marshalStats 将统计对象序列化。
func marshalStats(stats model.AnalysisStats) string {
	b, _ := json.Marshal(stats)
	return string(b)
}

// addAudit 记录审计事件（忽略写入失败）。
func (s *Service) addAudit(actor, action, targetType, targetID, detail string) {
	_ = s.st.Audit(actor, action, targetType, targetID, detail)
}
