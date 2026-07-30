package domain

import (
	"path/filepath"
	"strings"
)

// IsHighRiskOperationalPattern 判断模式是否描述破坏性、不可逆、特权或外部副作用类操作。
// 这类模式可以作为安全边界和检查入口，但不能与普通业务复用模式混合表达。
func IsHighRiskOperationalPattern(pattern Pattern) bool {
	text := patternSearchText(pattern)
	if text == "" {
		return false
	}
	if containsAny(text, highRiskPhrases) {
		return true
	}
	hasIrreversibleAction := containsAny(text, highRiskActions)
	hasSensitiveObject := containsAny(text, highRiskObjects)
	hasExternalEffect := containsAny(text, highRiskEffects)
	hasPathSignal := false
	for _, location := range pattern.EvidenceLocations {
		if pathHasHighRiskContext(location.Path) {
			hasPathSignal = true
			break
		}
	}
	if pattern.BusinessMethod != nil && pathHasHighRiskContext(pattern.BusinessMethod.DisplayLocation()) {
		hasPathSignal = true
	}
	return hasIrreversibleAction && (hasSensitiveObject || hasExternalEffect || hasPathSignal)
}

// HighRiskOperational 返回模板可直接读取的高风险操作标记。
func (p Pattern) HighRiskOperational() bool {
	return IsHighRiskOperationalPattern(p)
}

// IsRenderableNamingPattern 判断命名类模式是否足够稳定，能作为生成 reference 的入口。
func IsRenderableNamingPattern(pattern Pattern) bool {
	if pattern.AllowsHardConstraint() {
		return true
	}
	text := patternSearchText(pattern)
	if !containsAny(text, namingConventionTerms) {
		return false
	}
	return PatternEvidenceFileCount(pattern.EvidenceLocations) >= 2 || pattern.Frequency >= 2
}

// IsRenderableUtilityPattern 判断工具类模式是否能独立帮助未来开发定位或复用。
func IsRenderableUtilityPattern(pattern Pattern) bool {
	if pattern.AllowsHardConstraint() {
		return true
	}
	if PatternEvidenceFileCount(pattern.EvidenceLocations) == 0 {
		return false
	}
	text := patternSearchText(pattern)
	if containsAny(text, trivialUtilityTerms) && !containsAny(text, reusableUtilityContexts) {
		return false
	}
	return strings.TrimSpace(pattern.Description) != "" || strings.TrimSpace(pattern.Rule) != "" || pattern.BusinessMethod != nil
}

// IsRouteableUtilityFunction 判断 profile 中的工具函数是否值得生成到 common-utils。
func IsRouteableUtilityFunction(utility UtilityFunction) bool {
	name := strings.TrimSpace(utility.Name)
	file := strings.TrimSpace(utility.File)
	signature := strings.TrimSpace(utility.Signature)
	if name == "" || file == "" || signature == "" {
		return false
	}
	text := strings.ToLower(strings.Join([]string{name, signature, utility.Description, utility.Usage}, " "))
	hasUse := strings.TrimSpace(utility.Description) != "" || strings.TrimSpace(utility.Usage) != ""
	if !hasUse {
		return false
	}
	if containsAny(text, trivialUtilityTerms) && !containsAny(text, reusableUtilityContexts) {
		return false
	}
	return true
}

func patternSearchText(pattern Pattern) string {
	parts := []string{
		pattern.ID,
		pattern.Name,
		string(pattern.Category),
		pattern.Description,
		pattern.Rule,
		pattern.GoodExample,
		pattern.BadExample,
		pattern.ScopePath,
		pattern.WorkspaceRole,
	}
	if pattern.BusinessMethod != nil {
		parts = append(parts,
			pattern.BusinessMethod.Name,
			pattern.BusinessMethod.Description,
			pattern.BusinessMethod.Usage,
			pattern.BusinessMethod.Type,
			pattern.BusinessMethod.Function,
			pattern.BusinessMethod.Prerequisites,
			pattern.BusinessMethod.Returns,
			pattern.BusinessMethod.DisplayLocation(),
		)
	}
	for _, location := range pattern.EvidenceLocations {
		parts = append(parts, location.Path, location.Symbol, location.Kind, location.Description)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func containsAny(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func pathHasHighRiskContext(path string) bool {
	path = strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if path == "" {
		return false
	}
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == '.'
	})
	for _, part := range parts {
		switch part {
		case "cmd", "command", "commands", "script", "scripts", "migration", "migrations",
			"migrate", "bootstrap", "setup", "destroy", "cleanup", "reset", "purge":
			return true
		}
	}
	return false
}

var highRiskPhrases = []string{
	"drop table", "drop database", "drop schema", "delete database", "delete schema",
	"truncate table", "irreversible", "destructive", "production-affecting", "external side effect",
	"dangerous command", "privileged operation", "不可逆", "破坏性", "高风险", "外部副作用", "特权操作",
	"删表", "清库", "销毁命令",
}

var highRiskActions = []string{
	"destroy", "reset", "drop", "truncate", "purge", "wipe", "cleanup", "clean up",
	"delete", "remove", "revoke", "disable", "shutdown", "uninstall",
	"销毁", "重置", "删除", "清理", "清除", "撤销", "禁用", "关闭", "卸载",
}

var highRiskObjects = []string{
	"state", "lifecycle", "resource", "data", "storage", "schema", "credential", "secret", "token",
	"password", "key", "certificate", "cert", "license", "identity", "account", "role", "permission",
	"policy", "audit", "tenant", "payment", "billing", "session", "cache", "index", "artifact",
	"状态", "生命周期", "资源", "数据", "存储", "结构", "凭据", "密钥", "证书", "许可", "身份",
	"账号", "角色", "权限", "策略", "审计", "租户", "支付", "计费", "会话", "缓存", "索引", "产物",
}

var highRiskEffects = []string{
	"command", "cli", "script", "migration", "bootstrap", "setup", "release", "deploy", "publish",
	"production", "environment", "external", "remote", "network", "device", "service", "cluster",
	"infrastructure", "admin", "security", "命令", "脚本", "迁移", "初始化", "发布", "部署", "生产",
	"环境", "外部", "远程", "网络", "设备", "服务", "集群", "基础设施", "管理", "安全",
}

var namingConventionTerms = []string{
	"naming", "name", "suffix", "prefix", "camel", "snake", "kebab", "pascal", "acronym",
	"plural", "singular", "model", "entity", "component", "hook", "view", "page", "screen",
	"action", "event", "command", "job", "task", "request", "response", "adapter", "contract",
	"命名", "前缀", "后缀", "驼峰", "缩写", "模型", "实体", "组件", "视图", "页面", "动作",
	"事件", "命令", "任务", "请求", "响应", "适配", "契约",
}

var trivialUtilityTerms = []string{
	"cache key", "cachekey", "get key", "build key", "make key", "getter", "setter",
	"identity", "ptr", "pointer", "string helper", "format helper", "键名", "缓存键",
}

var reusableUtilityContexts = []string{
	"encrypt", "decrypt", "sign", "verify", "validate", "parse", "marshal", "unmarshal",
	"serialize", "deserialize", "encode", "decode", "transform", "map", "response", "error",
	"auth", "permission", "audit", "transaction", "retry", "lock", "config", "route", "request",
	"integration", "protocol", "normalize", "canonical", "format", "render", "state", "event",
	"加密", "解密", "签名", "验签", "校验", "验证", "解析", "序列化", "编码", "解码", "转换",
	"映射", "响应", "错误", "认证", "权限", "审计", "事务", "重试", "配置", "请求", "集成",
	"协议", "规范化", "格式化", "渲染", "状态", "事件",
}
