package generator

import "github.com/silaswei-io/skills-seed/internal/domain"

type categoryLinkRule struct {
	Category string
	ReasonZH string
	ReasonEN string
}

func categoryReferenceLinks(category string, allCategories []string, locale, pathPrefix string) []PatternReferenceLink {
	existing := make(map[string]bool, len(allCategories))
	for _, name := range allCategories {
		existing[name] = true
	}
	if pathPrefix == "" {
		pathPrefix = "./"
	}

	rules := relatedCategoryRules()[category]
	links := make([]PatternReferenceLink, 0, len(rules))
	for _, rule := range rules {
		if !existing[rule.Category] || rule.Category == category {
			continue
		}
		meta := categoryReferenceMetadata(rule.Category, locale)
		links = append(links, PatternReferenceLink{
			Title:  meta.Title,
			Path:   pathPrefix + rule.Category + ".md",
			Reason: localizedText(locale, rule.ReasonZH, rule.ReasonEN),
		})
	}
	return links
}

func businessPatternReferenceLinks(allCategories []string, locale, pathPrefix string) []PatternReferenceLink {
	return categoryReferenceLinks(string(domain.CategoryBusiness), allCategories, locale, pathPrefix)
}

func relatedCategoryRules() map[string][]categoryLinkRule {
	return map[string][]categoryLinkRule{
		string(domain.CategoryBusiness): {
			linkRule(domain.CategoryAPI,
				"涉及接口输入输出、响应字段、Extra/Before/After 数据或适配层返回时继续阅读。",
				"Read when the task touches API input/output, response fields, Extra/Before/After data, or adapter returns."),
			linkRule(domain.CategoryDatabase,
				"涉及持久化、事务、查询条件或数据模型字段落库时继续阅读。",
				"Read when the task touches persistence, transactions, query conditions, or stored model fields."),
			linkRule(domain.CategoryError,
				"涉及错误码、错误包装、失败分支或用户可见错误语义时继续阅读。",
				"Read when the task touches error codes, error wrapping, failure branches, or user-visible error semantics."),
		},
		string(domain.CategoryAPI): {
			linkRule(domain.CategoryBusiness,
				"接口字段承载业务状态、操作日志 Extra、Before/After 快照或跨实体规则时继续阅读。",
				"Read when API fields carry business state, operation-log Extra, Before/After snapshots, or cross-entity rules."),
			linkRule(domain.CategoryError,
				"接口需要返回错误码、错误响应结构或失败语义时继续阅读。",
				"Read when the API must return error codes, error response shapes, or failure semantics."),
			linkRule(domain.CategoryMiddleware,
				"接口行为依赖鉴权、拦截器、上下文注入或请求链路处理时继续阅读。",
				"Read when API behavior depends on auth, interceptors, context injection, or request pipeline handling."),
			linkRule(domain.CategoryConfig,
				"接口或派生产物受配置开关、路由配置或生成配置影响时继续阅读。",
				"Read when APIs or generated artifacts are controlled by feature flags, route config, or generation config."),
		},
		string(domain.CategoryDatabase): {
			linkRule(domain.CategoryBusiness,
				"数据库读写表达业务状态、生命周期、审计或跨实体约束时继续阅读。",
				"Read when database reads/writes express business state, lifecycle, audit, or cross-entity constraints."),
			linkRule(domain.CategoryConcurrency,
				"数据修改涉及锁、幂等、并发更新或一致性窗口时继续阅读。",
				"Read when data changes involve locks, idempotency, concurrent updates, or consistency windows."),
			linkRule(domain.CategoryError,
				"查询或事务失败需要特定错误语义、包装或降级处理时继续阅读。",
				"Read when query or transaction failures need specific error semantics, wrapping, or fallback handling."),
		},
		string(domain.CategoryMiddleware): {
			linkRule(domain.CategoryAPI,
				"中间件影响接口请求、响应、上下文或鉴权入口时继续阅读。",
				"Read when middleware affects API requests, responses, context, or auth entry points."),
			linkRule(domain.CategoryConfig,
				"中间件由配置开关、注册顺序或环境参数控制时继续阅读。",
				"Read when middleware is controlled by config flags, registration order, or environment parameters."),
			linkRule(domain.CategoryError,
				"中间件需要统一错误响应、拦截失败或恢复 panic 时继续阅读。",
				"Read when middleware must unify error responses, intercept failures, or recover panics."),
		},
		string(domain.CategoryConfig): {
			linkRule(domain.CategoryAPI,
				"配置影响接口、路由、派生产物或客户端契约时继续阅读。",
				"Read when config affects APIs, routes, generated artifacts, or client contracts."),
			linkRule(domain.CategoryMiddleware,
				"配置影响中间件启停、注册顺序或请求链路行为时继续阅读。",
				"Read when config affects middleware enablement, registration order, or request pipeline behavior."),
			linkRule(domain.CategoryDatabase,
				"配置影响数据库连接、迁移、表名、查询或持久化行为时继续阅读。",
				"Read when config affects database connections, migrations, table names, queries, or persistence behavior."),
		},
		string(domain.CategoryError): {
			linkRule(domain.CategoryAPI,
				"错误需要映射为接口响应、错误码或客户端可见结构时继续阅读。",
				"Read when errors must be mapped to API responses, error codes, or client-visible shapes."),
			linkRule(domain.CategoryBusiness,
				"错误分支表达业务规则、状态限制或审计语义时继续阅读。",
				"Read when failure branches express business rules, state constraints, or audit semantics."),
			linkRule(domain.CategoryDatabase,
				"错误来自查询、事务、唯一约束或持久化失败时继续阅读。",
				"Read when errors come from queries, transactions, unique constraints, or persistence failures."),
		},
		string(domain.CategoryUtils): {
			linkRule(domain.CategoryBusiness,
				"工具函数承载领域规则、审计数据或业务字段格式时继续阅读。",
				"Read when utility functions carry domain rules, audit data, or business field formatting."),
			linkRule(domain.CategoryAPI,
				"工具函数用于接口模型转换、响应字段或客户端契约时继续阅读。",
				"Read when utility functions are used for API model conversion, response fields, or client contracts."),
			linkRule(domain.CategoryTesting,
				"工具函数改动需要复用项目测试约定、fixture 或断言模式时继续阅读。",
				"Read when utility changes need project testing conventions, fixtures, or assertion patterns."),
		},
		string(domain.CategoryConcurrency): {
			linkRule(domain.CategoryDatabase,
				"并发控制影响数据一致性、事务边界或持久化顺序时继续阅读。",
				"Read when concurrency control affects data consistency, transaction boundaries, or persistence order."),
			linkRule(domain.CategoryBusiness,
				"锁、队列或异步流程承载业务状态变化时继续阅读。",
				"Read when locks, queues, or async flows carry business state transitions."),
			linkRule(domain.CategoryTesting,
				"并发行为需要 race、时序或重试类测试约定时继续阅读。",
				"Read when concurrent behavior needs race, timing, or retry-oriented test conventions."),
		},
		string(domain.CategoryTesting): {
			linkRule(domain.CategoryBusiness,
				"测试断言业务规则、状态流转或跨实体行为时继续阅读。",
				"Read when tests assert business rules, state transitions, or cross-entity behavior."),
			linkRule(domain.CategoryAPI,
				"测试覆盖接口契约、响应字段、生成代码或适配层时继续阅读。",
				"Read when tests cover API contracts, response fields, generated code, or adapters."),
			linkRule(domain.CategoryDatabase,
				"测试依赖数据库 fixture、事务隔离或持久化断言时继续阅读。",
				"Read when tests depend on database fixtures, transaction isolation, or persistence assertions."),
		},
		string(domain.CategoryStructure): {
			linkRule(domain.CategoryBusiness,
				"结构调整影响领域边界、入口编排或跨模块业务流时继续阅读。",
				"Read when structural changes affect domain boundaries, entry orchestration, or cross-module business flows."),
			linkRule(domain.CategoryAPI,
				"结构调整影响接口层、适配层、生成目录或契约文件时继续阅读。",
				"Read when structural changes affect API layers, adapters, generated directories, or contract files."),
			linkRule(domain.CategoryConfig,
				"结构调整需要同步配置路径、注册点或加载约定时继续阅读。",
				"Read when structural changes require config paths, registration points, or loading conventions."),
		},
		string(domain.CategoryNaming): {
			linkRule(domain.CategoryAPI,
				"命名影响接口字段、路径、错误码或客户端契约时继续阅读。",
				"Read when naming affects API fields, paths, error codes, or client contracts."),
			linkRule(domain.CategoryDatabase,
				"命名影响表、列、索引、查询方法或持久化模型时继续阅读。",
				"Read when naming affects tables, columns, indexes, query methods, or persistence models."),
			linkRule(domain.CategoryBusiness,
				"命名承载领域概念、状态名称或业务动作时继续阅读。",
				"Read when naming carries domain concepts, state names, or business actions."),
		},
	}
}

func linkRule(category domain.Category, reasonZH, reasonEN string) categoryLinkRule {
	return categoryLinkRule{
		Category: string(category),
		ReasonZH: reasonZH,
		ReasonEN: reasonEN,
	}
}
