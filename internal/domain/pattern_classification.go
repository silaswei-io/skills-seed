package domain

import (
	"sort"
	"strings"
)

const (
	// KnowledgeFlagOperationalRisk 标识不可逆、特权、状态或资源变更以及外部可见行为。
	KnowledgeFlagOperationalRisk = "operational_risk"
)

// ValidKnowledgeFlags 报告所有知识标志是否属于受控集合。
func ValidKnowledgeFlags(flags []string) bool {
	for _, flag := range flags {
		if strings.TrimSpace(flag) != KnowledgeFlagOperationalRisk {
			return false
		}
	}
	return true
}

// CanonicalKnowledgeFlags 返回去重、排序后的合法知识标志。
func CanonicalKnowledgeFlags(flags []string) []string {
	seen := make(map[string]struct{}, len(flags))
	for _, flag := range flags {
		flag = strings.TrimSpace(flag)
		if flag != KnowledgeFlagOperationalRisk {
			continue
		}
		seen[flag] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for flag := range seen {
		out = append(out, flag)
	}
	sort.Strings(out)
	return out
}

// MergeKnowledgeFlags 合并多个已校验来源的知识标志。
func MergeKnowledgeFlags(groups ...[]string) []string {
	var merged []string
	for _, group := range groups {
		merged = append(merged, group...)
	}
	return CanonicalKnowledgeFlags(merged)
}

// HasKnowledgeFlag 判断模式是否带有指定受控标志。
func (p Pattern) HasKnowledgeFlag(flag string) bool {
	for _, candidate := range p.KnowledgeFlags {
		if candidate == flag {
			return true
		}
	}
	return false
}

// HighRiskOperational 返回模板可直接读取的操作风险标记。
func (p Pattern) HighRiskOperational() bool {
	return p.HasKnowledgeFlag(KnowledgeFlagOperationalRisk)
}
