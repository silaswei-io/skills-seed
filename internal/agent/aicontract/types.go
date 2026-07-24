package aicontract

// CodeLocationOutput 是 AI 输出中的最小可维护代码位置契约。
type CodeLocationOutput struct {
	CurrentLocation string `json:"current_location" jsonschema:"required,example=src/component/file.ext:140" jsonschema_description:"repository-relative source file and 1-based line in the format src/component/file.ext:140"`
}

// EvidenceLocationOutput 描述模式对应的源码证据位置。
type EvidenceLocationOutput struct {
	Path        string  `json:"path,omitempty" jsonschema_description:"relative/file/path"`
	Line        int     `json:"line,omitempty" jsonschema:"minimum=1" jsonschema_description:"single 1-based line number"`
	Symbol      string  `json:"symbol,omitempty" jsonschema_description:"real function, method, type, class, interface, struct, enum, trait, protocol, module, object, or file name"`
	Kind        string  `json:"kind,omitempty" jsonschema:"enum=function,enum=func,enum=method,enum=type,enum=class,enum=interface,enum=struct,enum=enum,enum=trait,enum=protocol,enum=module,enum=object,enum=file" jsonschema_description:"canonical source symbol kind; omit when unknown"`
	Description string  `json:"description,omitempty" jsonschema_description:"how this location supports the pattern"`
	Confidence  float64 `json:"confidence,omitempty" jsonschema:"minimum=0,maximum=1" jsonschema_description:"0.0-1.0"`
}

// BusinessMethodOutput 描述可路由、可复用的业务或工具入口。
type BusinessMethodOutput struct {
	Name          string             `json:"name" jsonschema_description:"qualified code identifier only; do not put a signature in this field"`
	CodeLocation  CodeLocationOutput `json:"code_location" jsonschema_description:"object containing current_location; never output code_location as a single string"`
	Description   string             `json:"description" jsonschema_description:"capability and boundary supported by source evidence"`
	Usage         string             `json:"usage" jsonschema_description:"applicability and when to locate or call this canonical reusable entry"`
	Type          string             `json:"type" jsonschema:"enum=domain,enum=common" jsonschema_description:"domain|common"`
	Function      string             `json:"function" jsonschema_description:"complete signature with parameters and return types"`
	Prerequisites string             `json:"prerequisites" jsonschema_description:"required setup or dependencies as one string"`
	Returns       string             `json:"returns" jsonschema_description:"return values and error semantics as one string"`
}

// PatternOutput 是所有模式学习类 prompt 的基础输出契约。
type PatternOutput struct {
	ID                string                   `json:"id" jsonschema_description:"kebab-case-id"`
	Name              string                   `json:"name" jsonschema_description:"pattern name"`
	Category          string                   `json:"category" jsonschema:"enum=naming,enum=error,enum=structure,enum=concurrency,enum=testing,enum=business,enum=api,enum=database,enum=utils,enum=middleware,enum=config" jsonschema_description:"one allowed category"`
	Description       string                   `json:"description" jsonschema_description:"source-backed problem or capability, matching triggers, observed behavior, and applicability boundary; for multiple evidence locations include only behavior every location proves"`
	GoodExample       string                   `json:"good_example" jsonschema_description:"source-backed code evidence as an escaped JSON string"`
	BadExample        string                   `json:"bad_example" jsonschema_description:"common mistake to avoid or empty string"`
	Rule              string                   `json:"rule" jsonschema_description:"non-mandatory reuse guidance naming verified existing entries or boundaries to prefer and when extension is appropriate; never generalize beyond source evidence"`
	Confidence        float64                  `json:"confidence" jsonschema:"minimum=0,maximum=1" jsonschema_description:"0.0-1.0"`
	Frequency         int                      `json:"frequency" jsonschema:"minimum=1" jsonschema_description:"positive integer occurrence count"`
	AnalysisUnitID    string                   `json:"analysis_unit_id,omitempty" jsonschema_description:"input analysis unit id"`
	AnalysisUnitName  string                   `json:"analysis_unit_name,omitempty" jsonschema_description:"input analysis unit name"`
	EvidenceLocations []EvidenceLocationOutput `json:"evidence_locations,omitempty" jsonschema_description:"minimum source-backed implementation chain needed for correct reuse"`
	BusinessMethod    *BusinessMethodOutput    `json:"business_method,omitempty" jsonschema_description:"canonical reusable callable entry with verified signature and location, or null"`
}

// CuratedPatternOutput 只承载 AI 拥有的规范文本和来源决策。
type CuratedPatternOutput struct {
	ID          string   `json:"id" jsonschema_description:"canonical id, preferably an input id"`
	Name        string   `json:"name" jsonschema_description:"canonical pattern name"`
	Category    string   `json:"category" jsonschema:"enum=naming,enum=error,enum=structure,enum=concurrency,enum=testing,enum=business,enum=api,enum=database,enum=utils,enum=middleware,enum=config" jsonschema_description:"one allowed category"`
	Description string   `json:"description" jsonschema_description:"factual solved problem, matching triggers, observed behavior, and applicability boundary"`
	Rule        string   `json:"rule" jsonschema_description:"canonical reuse guidance; normative only for explicit authoritative input"`
	Confidence  float64  `json:"confidence" jsonschema:"minimum=0,maximum=1" jsonschema_description:"0.0-1.0 evidence consistency and specificity"`
	SourceIDs   []string `json:"source_ids" jsonschema_description:"all real candidate or existing pattern ids represented by this canonical pattern"`
}

type ValidationCommandOutput struct {
	Command    string   `json:"command" jsonschema_description:"exact full validation command shown by evidence"`
	When       string   `json:"when,omitempty" jsonschema_description:"when to run this command"`
	Source     string   `json:"source,omitempty" jsonschema_description:"repository-relative evidence path or user_context"`
	Workdir    string   `json:"workdir,omitempty" jsonschema_description:"repository-relative workdir, empty for project root"`
	ScopePaths []string `json:"scope_paths,omitempty" jsonschema_description:"relative paths or directories explicitly covered by this command; leave empty for broad or unclear commands"`
	Evidence   []string `json:"evidence,omitempty" jsonschema_description:"relative repository paths proving this command and its scope; do not use unconfirmed placeholders"`
	Type       string   `json:"type,omitempty" jsonschema:"enum=test,enum=build,enum=lint,enum=generate,enum=contract,enum=check" jsonschema_description:"test|build|lint|generate|contract|check"`
}

type EngineeringRuleOutput struct {
	Title    string   `json:"title" jsonschema_description:"concise authoritative rule title"`
	Rule     string   `json:"rule" jsonschema_description:"exact actionable constraint supported by the authoritative source"`
	Source   string   `json:"source" jsonschema_description:"repository-relative authoritative engineering knowledge path or user_context"`
	Evidence []string `json:"evidence,omitempty" jsonschema_description:"repository-relative files supporting the constraint"`
}

type ArchitectureLayerOutput struct {
	Name             string   `json:"name" jsonschema_description:"real responsibility layer name"`
	Description      string   `json:"description" jsonschema_description:"layer responsibility"`
	Responsibilities []string `json:"responsibilities" jsonschema_description:"specific responsibilities"`
	Files            []string `json:"files" jsonschema_description:"relative files owned by this layer"`
}

type UtilityFunctionOutput struct {
	Name        string `json:"name" jsonschema_description:"utility name"`
	File        string `json:"file" jsonschema_description:"real repository-relative file path or external package path; omit the utility if the location is unconfirmed"`
	Signature   string `json:"signature" jsonschema_description:"real function or method signature"`
	Description string `json:"description" jsonschema_description:"domain-neutral utility responsibility; external dependency interaction entries that carry product-domain behavior should prefer business_methods"`
	Usage       string `json:"usage" jsonschema_description:"when to use it"`
}

type ModuleOutput struct {
	Name             string   `json:"name" jsonschema_description:"module name"`
	Path             string   `json:"path" jsonschema_description:"real repository-relative directory or file path; omit the module if no concrete path is confirmed"`
	Description      string   `json:"description" jsonschema_description:"module responsibility"`
	Responsibilities []string `json:"responsibilities" jsonschema_description:"specific responsibilities"`
	Dependencies     []string `json:"dependencies" jsonschema_description:"concrete module names or paths directly evidenced by imports, calls, registration, configuration, or an existing still-supported profile"`
	Dependents       []string `json:"dependents" jsonschema_description:"concrete dependent module names or paths supported by the same direct evidence"`
	KeyMethods       []string `json:"key_methods" jsonschema_description:"real methods or entries"`
}

// ProjectProfileOutput 是完整项目画像的 AI 输出契约。
type ProjectProfileOutput struct {
	ProjectName        string                    `json:"project_name" jsonschema_description:"project name"`
	Language           string                    `json:"language" jsonschema_description:"primary language"`
	Frameworks         []string                  `json:"frameworks" jsonschema_description:"framework or runtime names supported by evidence"`
	Architecture       string                    `json:"architecture" jsonschema_description:"concrete architecture description"`
	Layers             []ArchitectureLayerOutput `json:"layers" jsonschema_description:"architecture layers"`
	DependencyGraph    string                    `json:"dependency_graph" jsonschema_description:"dependency direction using real module names"`
	DataFlow           string                    `json:"data_flow" jsonschema_description:"data flow using real processing stages"`
	FrameworkPatterns  []string                  `json:"framework_patterns" jsonschema_description:"concrete framework usage patterns"`
	Structure          string                    `json:"structure" jsonschema_description:"concrete project structure summary"`
	KeyModules         []ModuleOutput            `json:"key_modules" jsonschema_description:"key modules"`
	BusinessMethods    []BusinessMethodOutput    `json:"business_methods" jsonschema_description:"project-level reusable entry points"`
	CommonUtils        []UtilityFunctionOutput   `json:"common_utils" jsonschema_description:"domain-neutral utility functions"`
	ConfigPatterns     []string                  `json:"config_patterns" jsonschema_description:"configuration conventions"`
	Dependencies       []string                  `json:"dependencies" jsonschema_description:"important dependencies"`
	ValidationCommands []ValidationCommandOutput `json:"validation_commands" jsonschema_description:"repository-evidenced validation commands"`
	EngineeringRules   []EngineeringRuleOutput   `json:"engineering_rules" jsonschema_description:"explicit constraints from authoritative engineering knowledge or user context"`
	Summary            string                    `json:"summary" jsonschema_description:"specific project overview"`
}

type ProfileRefreshRecommendationOutput struct {
	Needed bool   `json:"needed" jsonschema_description:"true only when broad structure or technology changes require full refresh"`
	Reason string `json:"reason,omitempty" jsonschema_description:"reason when refresh is needed"`
}

type SelectFilesOutput struct {
	Include       []string `json:"include" jsonschema_description:"relative paths or broad globs to include"`
	Exclude       []string `json:"exclude" jsonschema_description:"relative paths or globs to exclude"`
	SelectedPaths []string `json:"selected_paths" jsonschema_description:"explicit relative file paths sorted lexicographically"`
	Reason        string   `json:"reason" jsonschema_description:"brief generic rationale"`
}

type AnalyzeIssueOutput struct {
	File       string `json:"file" jsonschema_description:"relative file path"`
	Line       int    `json:"line" jsonschema:"minimum=1" jsonschema_description:"single 1-based line number"`
	Severity   string `json:"severity" jsonschema:"enum=error,enum=warning,enum=info" jsonschema_description:"error|warning|info"`
	Message    string `json:"message" jsonschema_description:"issue description"`
	Suggestion string `json:"suggestion" jsonschema_description:"specific fix suggestion"`
	PatternID  string `json:"pattern_id" jsonschema_description:"matched pattern id or empty string"`
}

type AnalyzeCodeOutput struct {
	Issues      []AnalyzeIssueOutput `json:"issues" jsonschema_description:"found issues, empty array when none"`
	Suggestions []string             `json:"suggestions" jsonschema_description:"general improvement suggestions"`
	Confidence  float64              `json:"confidence" jsonschema:"minimum=0,maximum=1" jsonschema_description:"0.0-1.0"`
}

type GenerateFixesOutput struct {
	Fixes      map[string]string `json:"fixes" jsonschema_description:"complete fixed content for that file"`
	Confidence float64           `json:"confidence" jsonschema:"minimum=0,maximum=1" jsonschema_description:"0.0-1.0"`
	Summary    string            `json:"summary" jsonschema_description:"overall fix summary or empty string"`
	Warnings   []string          `json:"warnings" jsonschema_description:"manual review warnings"`
}

type LearnPatternsOutput struct {
	Patterns []PatternOutput `json:"patterns" jsonschema_description:"learned patterns"`
}

type CuratePatternsOutput struct {
	Patterns []CuratedPatternOutput `json:"patterns" jsonschema_description:"final canonical patterns to write"`
	Dropped  []CuratedDropOutput    `json:"dropped" jsonschema_description:"candidate patterns not written"`
}

type CuratedDropOutput struct {
	ID     string `json:"id" jsonschema_description:"candidate id"`
	Reason string `json:"reason" jsonschema_description:"why it should not be written"`
}

type AnalyzeCurrentCodebaseOutput struct {
	Patterns                  []PatternOutput                    `json:"patterns" jsonschema_description:"candidate patterns"`
	ProfileRefreshRecommended ProfileRefreshRecommendationOutput `json:"profile_refresh_recommended" jsonschema_description:"whether a full project profile refresh is needed"`
}

type AnalyzeCurrentCodebaseBatchOutput struct {
	Units []AnalyzeCurrentCodebaseBatchUnitOutput `json:"units" jsonschema_description:"one result object for every evidenced input unit"`
}

type AnalyzeCurrentCodebaseBatchUnitOutput struct {
	UnitID                    string                             `json:"unit_id" jsonschema_description:"input unit id"`
	UnitName                  string                             `json:"unit_name" jsonschema_description:"input unit name"`
	Patterns                  []PatternOutput                    `json:"patterns" jsonschema_description:"candidate patterns for this unit"`
	ProfileRefreshRecommended ProfileRefreshRecommendationOutput `json:"profile_refresh_recommended" jsonschema_description:"whether this unit requires full profile refresh"`
}

type AnalysisUnitOutput struct {
	ID           string   `json:"id" jsonschema_description:"kebab-case-id"`
	Name         string   `json:"name" jsonschema_description:"business analysis unit name"`
	RouteTerms   []string `json:"route_terms,omitempty" jsonschema_description:"requirement, state, action, resource, or external system terms"`
	EntryPaths   []string `json:"entry_paths,omitempty" jsonschema_description:"paths relative to project root from the allowed file list"`
	RelatedPaths []string `json:"related_paths,omitempty" jsonschema_description:"paths relative to project root from the allowed file list"`
	ScopeReason  string   `json:"scope_reason,omitempty" jsonschema_description:"why these files belong together"`
}

type PlanAnalysisUnitsOutput struct {
	Units []AnalysisUnitOutput `json:"units" jsonschema_description:"business analysis units"`
}

type WorkspaceProjectAnalysisOutput struct {
	ProjectID      string   `json:"project_id" jsonschema_description:"exact configured project id"`
	Responsibility string   `json:"responsibility,omitempty" jsonschema_description:"specific responsibility"`
	Frameworks     []string `json:"frameworks,omitempty" jsonschema_description:"frameworks or runtimes supported by evidence"`
}

type WorkspacePathOutput struct {
	Path             string   `json:"path" jsonschema_description:"concrete existing workspace-root file or directory; not a glob pattern, child-project-local path, platform name, registry name, URL, Jenkins job, or external endpoint"`
	Description      string   `json:"description,omitempty" jsonschema_description:"path responsibility"`
	Consumers        []string `json:"consumers,omitempty" jsonschema_description:"consumer project ids"`
	Producers        []string `json:"producers,omitempty" jsonschema_description:"producer project ids"`
	AffectedProjects []string `json:"affected_projects,omitempty" jsonschema_description:"affected project ids"`
}

type WorkspaceDependencyOutput struct {
	FromProjectID string                   `json:"from_project_id" jsonschema_description:"exact configured source project id"`
	To            WorkspaceReferenceOutput `json:"to" jsonschema_description:"typed target; use project for configured child projects and path only for declared shared, contract, or infrastructure paths"`
	Reason        string                   `json:"reason" jsonschema_description:"evidenced dependency reason"`
}

type WorkspaceReferenceOutput struct {
	Kind  string `json:"kind" jsonschema:"enum=project,enum=role,enum=path" jsonschema_description:"project|role|path"`
	Value string `json:"value" jsonschema_description:"exact configured project id, configured role, or declared workspace-relative path/pattern; do not include project: or path: helper prefixes"`
}

type WorkspaceRouteOutput struct {
	PathPattern string   `json:"path_pattern" jsonschema_description:"path glob relative to workspace root"`
	ProjectIDs  []string `json:"project_ids" jsonschema_description:"affected project ids"`
	Reason      string   `json:"reason" jsonschema_description:"why this path affects those projects"`
}

type WorkspaceProfileOutput struct {
	Summary      string                           `json:"summary,omitempty" jsonschema_description:"workspace-level factual summary"`
	Projects     []WorkspaceProjectAnalysisOutput `json:"projects" jsonschema_description:"analysis fields keyed by exact configured project id; identity fields come only from config"`
	Shared       []WorkspacePathOutput            `json:"shared,omitempty" jsonschema_description:"shared paths"`
	Contracts    []WorkspacePathOutput            `json:"contracts,omitempty" jsonschema_description:"contract paths"`
	Infra        []WorkspacePathOutput            `json:"infra,omitempty" jsonschema_description:"infrastructure paths"`
	Dependencies []WorkspaceDependencyOutput      `json:"dependencies,omitempty" jsonschema_description:"workspace dependencies"`
	ImpactRoutes []WorkspaceRouteOutput           `json:"impact_routes,omitempty" jsonschema_description:"impact routes"`
}

type WorkspaceRuleOutput struct {
	Title       string                     `json:"title" jsonschema_description:"rule title"`
	Description string                     `json:"description" jsonschema_description:"actionable workspace rule"`
	AppliesTo   []WorkspaceReferenceOutput `json:"applies_to,omitempty" jsonschema_description:"typed project, role, or path scopes"`
	Source      string                     `json:"source" jsonschema_description:"workspace_profile, user_context, or a repository-relative authoritative source path"`
	Evidence    []string                   `json:"evidence,omitempty" jsonschema_description:"repository-relative files supporting the rule"`
}

type WorkspaceParallelGuidanceOutput struct {
	Scope     WorkspaceReferenceOutput `json:"scope" jsonschema_description:"typed project, role, or path scope"`
	Allowed   bool                     `json:"allowed" jsonschema_description:"true when concurrent work is allowed"`
	Condition string                   `json:"condition" jsonschema_description:"condition for safe parallelism"`
}

type WorkspaceLoadMultipleSkillOutput struct {
	Condition  string   `json:"condition" jsonschema_description:"when multiple child skills are needed"`
	ProjectIDs []string `json:"project_ids" jsonschema_description:"child project ids to load"`
	Reason     string   `json:"reason" jsonschema_description:"why these skills are needed together"`
}

type WorkspaceSpecOutput struct {
	Routing                []WorkspaceRouteOutput             `json:"routing" jsonschema_description:"routing rules"`
	Rules                  []WorkspaceRuleOutput              `json:"rules" jsonschema_description:"workspace-level rules"`
	ChangeOrder            []string                           `json:"change_order,omitempty" jsonschema_description:"ordered step text without numeric or list prefixes; presentation adds numbering"`
	ParallelAgentGuidance  []WorkspaceParallelGuidanceOutput  `json:"parallel_agent_guidance,omitempty" jsonschema_description:"parallel agent boundaries"`
	LoadMultipleSkillsWhen []WorkspaceLoadMultipleSkillOutput `json:"load_multiple_skills_when,omitempty" jsonschema_description:"when to load multiple child skills"`
}

type OptimizeWorkflowOutput struct {
	Title     string   `json:"title" jsonschema_description:"short title"`
	Content   string   `json:"content" jsonschema_description:"complete Markdown workflow body"`
	Conflicts []string `json:"conflicts" jsonschema_description:"incompatible existing and new requirements; empty when the workflow can be saved"`
}
