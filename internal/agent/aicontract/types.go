package aicontract

// CodeLocationOutput 是 AI 输出中的最小可维护代码位置契约。
type CodeLocationOutput struct {
	CurrentLocation string `json:"current_location,omitempty" jsonschema:"example=packages/modules/event-bus-local/src/services/event-bus-local.ts:140" jsonschema_description:"repository-relative source file and 1-based line in the format packages/modules/event-bus-local/src/services/event-bus-local.ts:140"`
}

// EvidenceLocationOutput 描述模式对应的源码证据位置。
type EvidenceLocationOutput struct {
	Path        string  `json:"path,omitempty" jsonschema_description:"relative/file/path"`
	Line        int     `json:"line,omitempty" jsonschema_description:"single 1-based line number"`
	Symbol      string  `json:"symbol,omitempty" jsonschema_description:"real function, method, type, variable, or file name"`
	Kind        string  `json:"kind,omitempty" jsonschema_description:"function|method|type|file"`
	Description string  `json:"description,omitempty" jsonschema_description:"how this location supports the pattern"`
	Confidence  float64 `json:"confidence,omitempty" jsonschema_description:"0.0-1.0"`
}

// BusinessMethodOutput 描述可路由、可复用的业务或工具入口。
type BusinessMethodOutput struct {
	Name          string             `json:"name" jsonschema_description:"complete method signature or code identifier"`
	CodeLocation  CodeLocationOutput `json:"code_location" jsonschema_description:"object containing current_location; never output code_location as a single string"`
	Description   string             `json:"description" jsonschema_description:"responsibility supported by source evidence"`
	Usage         string             `json:"usage" jsonschema_description:"when to locate or call this entry point"`
	Type          string             `json:"type" jsonschema_description:"domain|common"`
	Function      string             `json:"function" jsonschema_description:"complete signature with parameters and return types"`
	Prerequisites string             `json:"prerequisites" jsonschema_description:"required setup or dependencies as one string"`
	Returns       string             `json:"returns" jsonschema_description:"return values and error semantics as one string"`
}

// PatternOutput 是所有模式学习类 prompt 的基础输出契约。
type PatternOutput struct {
	ID                string                   `json:"id" jsonschema_description:"kebab-case-id"`
	Name              string                   `json:"name" jsonschema_description:"pattern name"`
	Category          string                   `json:"category" jsonschema_description:"one allowed category"`
	Description       string                   `json:"description" jsonschema_description:"what this pattern does and when to use it"`
	GoodExample       string                   `json:"good_example" jsonschema_description:"source-backed code evidence as an escaped JSON string"`
	BadExample        string                   `json:"bad_example" jsonschema_description:"common mistake to avoid or empty string"`
	Rule              string                   `json:"rule" jsonschema_description:"actionable rule saying when to apply the pattern"`
	Confidence        float64                  `json:"confidence" jsonschema_description:"0.0-1.0"`
	Frequency         int                      `json:"frequency" jsonschema_description:"integer occurrence count"`
	AnalysisUnitID    string                   `json:"analysis_unit_id,omitempty" jsonschema_description:"input analysis unit id"`
	AnalysisUnitName  string                   `json:"analysis_unit_name,omitempty" jsonschema_description:"input analysis unit name"`
	EvidenceLocations []EvidenceLocationOutput `json:"evidence_locations,omitempty" jsonschema_description:"pattern-level source evidence locations"`
	BusinessMethod    *BusinessMethodOutput    `json:"business_method,omitempty" jsonschema_description:"real callable business or utility method, or null when not applicable"`
}

// CuratedPatternOutput 是模式策展 prompt 的输出契约。
type CuratedPatternOutput struct {
	ID                string                   `json:"id" jsonschema_description:"existing or candidate pattern id"`
	Name              string                   `json:"name" jsonschema_description:"canonical pattern name"`
	Category          string                   `json:"category" jsonschema_description:"one allowed category"`
	Description       string                   `json:"description" jsonschema_description:"clear applicability boundary"`
	GoodExample       string                   `json:"good_example" jsonschema_description:"real code example selected from input"`
	BadExample        string                   `json:"bad_example" jsonschema_description:"bad example selected from input, or empty string"`
	Rule              string                   `json:"rule" jsonschema_description:"canonical actionable rule"`
	Confidence        float64                  `json:"confidence" jsonschema_description:"0.0-1.0"`
	Frequency         int                      `json:"frequency" jsonschema_description:"integer occurrence count"`
	EvidenceLocations []EvidenceLocationOutput `json:"evidence_locations,omitempty" jsonschema_description:"evidence locations preserved from input"`
	MergedFrom        []string                 `json:"merged_from" jsonschema_description:"merged candidate or existing pattern ids"`
	MergeReason       string                   `json:"merge_reason" jsonschema_description:"why this pattern is added, updated, or merged"`
	SimilarityScore   float64                  `json:"similarity_score" jsonschema_description:"0.0-1.0"`
	Source            string                   `json:"source" jsonschema_description:"learned_current|learned_history|user_defined"`
	BusinessMethod    *BusinessMethodOutput    `json:"business_method,omitempty" jsonschema_description:"business method preserved from input, or null when not applicable"`
	ProjectID         string                   `json:"project_id,omitempty" jsonschema_description:"workspace project id"`
	ScopePath         string                   `json:"scope_path,omitempty" jsonschema_description:"relative workspace scope path"`
	WorkspaceRole     string                   `json:"workspace_role,omitempty" jsonschema_description:"workspace role"`
	AnalysisUnitID    string                   `json:"analysis_unit_id,omitempty" jsonschema_description:"analysis unit id preserved from input"`
	AnalysisUnitName  string                   `json:"analysis_unit_name,omitempty" jsonschema_description:"analysis unit name preserved from input"`
}

type ValidationCommandOutput struct {
	Command    string   `json:"command" jsonschema_description:"exact full validation command shown by evidence"`
	When       string   `json:"when,omitempty" jsonschema_description:"when to run this command"`
	Source     string   `json:"source,omitempty" jsonschema_description:"repository-relative evidence path or user_context"`
	Workdir    string   `json:"workdir,omitempty" jsonschema_description:"repository-relative workdir, empty for project root"`
	ScopePaths []string `json:"scope_paths,omitempty" jsonschema_description:"relative paths or directories explicitly covered by this command; leave empty for broad or unclear commands"`
	Evidence   []string `json:"evidence,omitempty" jsonschema_description:"relative repository paths proving this command and its scope; do not use unconfirmed placeholders"`
	Type       string   `json:"type,omitempty" jsonschema_description:"test|build|lint|generate|contract|check"`
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
	Dependencies     []string `json:"dependencies" jsonschema_description:"dependency modules"`
	Dependents       []string `json:"dependents" jsonschema_description:"dependent modules"`
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
	Summary            string                    `json:"summary" jsonschema_description:"specific project overview"`
}

// ProjectProfileDeltaOutput 是当前代码学习阶段允许返回的画像增量。
type ProjectProfileDeltaOutput struct {
	Frameworks         []string                  `json:"frameworks,omitempty" jsonschema_description:"new framework or runtime confirmed by evidence"`
	Dependencies       []string                  `json:"dependencies,omitempty" jsonschema_description:"new dependency or tool confirmed by evidence"`
	Layers             []ArchitectureLayerOutput `json:"layers,omitempty" jsonschema_description:"architecture layer deltas"`
	KeyModules         []ModuleOutput            `json:"key_modules,omitempty" jsonschema_description:"key module deltas"`
	CommonUtils        []UtilityFunctionOutput   `json:"common_utils,omitempty" jsonschema_description:"utility deltas"`
	ConfigPatterns     []string                  `json:"config_patterns,omitempty" jsonschema_description:"configuration conventions confirmed by this run"`
	FrameworkPatterns  []string                  `json:"framework_patterns,omitempty" jsonschema_description:"framework usage conventions confirmed by this run"`
	BusinessMethods    []BusinessMethodOutput    `json:"business_methods,omitempty" jsonschema_description:"new reusable business or utility entries"`
	ValidationCommands []ValidationCommandOutput `json:"validation_commands,omitempty" jsonschema_description:"new repository-evidenced validation commands"`
	Summary            string                    `json:"summary,omitempty" jsonschema_description:"concise delta summary"`
	Architecture       string                    `json:"architecture,omitempty" jsonschema_description:"architecture delta only"`
	Structure          string                    `json:"structure,omitempty" jsonschema_description:"structure delta only"`
	DependencyGraph    string                    `json:"dependency_graph,omitempty" jsonschema_description:"dependency graph delta only"`
	DataFlow           string                    `json:"data_flow,omitempty" jsonschema_description:"data flow delta only"`
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
	Line       int    `json:"line" jsonschema_description:"single 1-based line number"`
	Severity   string `json:"severity" jsonschema_description:"error|warning|info"`
	Message    string `json:"message" jsonschema_description:"issue description"`
	Suggestion string `json:"suggestion" jsonschema_description:"specific fix suggestion"`
	PatternID  string `json:"pattern_id" jsonschema_description:"matched pattern id or empty string"`
}

type AnalyzeCodeOutput struct {
	Issues      []AnalyzeIssueOutput `json:"issues" jsonschema_description:"found issues, empty array when none"`
	Suggestions []string             `json:"suggestions" jsonschema_description:"general improvement suggestions"`
	Confidence  float64              `json:"confidence" jsonschema_description:"0.0-1.0"`
}

type GenerateFixesOutput struct {
	Fixes      map[string]string `json:"fixes" jsonschema_description:"complete fixed content for that file"`
	Confidence float64           `json:"confidence" jsonschema_description:"0.0-1.0"`
	Summary    string            `json:"summary" jsonschema_description:"overall fix summary or empty string"`
	Warnings   []string          `json:"warnings" jsonschema_description:"manual review warnings"`
}

type LearnPatternsOutput struct {
	Patterns []PatternOutput `json:"patterns" jsonschema_description:"learned patterns"`
}

type CuratePatternsOutput struct {
	Patterns []CuratedPatternOutput `json:"patterns" jsonschema_description:"final canonical patterns to write"`
	Dropped  []CuratedDropOutput    `json:"dropped" jsonschema_description:"candidate patterns not written"`
	Summary  CurateSummaryOutput    `json:"summary" jsonschema_description:"curation statistics"`
}

type CuratedDropOutput struct {
	ID     string `json:"id" jsonschema_description:"candidate id"`
	Reason string `json:"reason" jsonschema_description:"why it should not be written"`
}

type CurateSummaryOutput struct {
	TotalCandidates int `json:"total_candidates" jsonschema_description:"candidate count"`
	TotalExisting   int `json:"total_existing" jsonschema_description:"related existing count"`
	TotalWritten    int `json:"total_written" jsonschema_description:"len(patterns)"`
	TotalDropped    int `json:"total_dropped" jsonschema_description:"len(dropped)"`
	MergeCount      int `json:"merge_count" jsonschema_description:"merge operation count"`
}

type AnalyzeCurrentCodebaseOutput struct {
	Patterns                  []PatternOutput                    `json:"patterns" jsonschema_description:"candidate patterns"`
	ProfileDelta              ProjectProfileDeltaOutput          `json:"profile_delta" jsonschema_description:"structured project facts newly added or changed by evidence"`
	ProfileRefreshRecommended ProfileRefreshRecommendationOutput `json:"profile_refresh_recommended" jsonschema_description:"whether a full project profile refresh is needed"`
}

type AnalyzeCurrentCodebaseBatchOutput struct {
	Units []AnalyzeCurrentCodebaseBatchUnitOutput `json:"units" jsonschema_description:"one result object for every evidenced input unit"`
}

type AnalyzeCurrentCodebaseBatchUnitOutput struct {
	UnitID                    string                             `json:"unit_id" jsonschema_description:"input unit id"`
	UnitName                  string                             `json:"unit_name" jsonschema_description:"input unit name"`
	Patterns                  []PatternOutput                    `json:"patterns" jsonschema_description:"candidate patterns for this unit"`
	ProfileDelta              ProjectProfileDeltaOutput          `json:"profile_delta" jsonschema_description:"structured project facts for this unit"`
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

type WorkspaceProjectOutput struct {
	ID             string   `json:"id" jsonschema_description:"project id"`
	Path           string   `json:"path" jsonschema_description:"project relative path"`
	Type           string   `json:"type" jsonschema_description:"application|library|service|tooling or evidenced project type"`
	Language       string   `json:"language" jsonschema_description:"primary language"`
	Responsibility string   `json:"responsibility,omitempty" jsonschema_description:"specific responsibility"`
	Frameworks     []string `json:"frameworks,omitempty" jsonschema_description:"frameworks or runtimes supported by evidence"`
}

type WorkspacePathOutput struct {
	Path             string   `json:"path" jsonschema_description:"workspace relative path"`
	Description      string   `json:"description,omitempty" jsonschema_description:"path responsibility"`
	Consumers        []string `json:"consumers,omitempty" jsonschema_description:"consumer project ids"`
	Producers        []string `json:"producers,omitempty" jsonschema_description:"producer project ids"`
	AffectedProjects []string `json:"affected_projects,omitempty" jsonschema_description:"affected project ids"`
}

type WorkspaceDependencyOutput struct {
	From   string `json:"from" jsonschema_description:"source project id"`
	To     string `json:"to" jsonschema_description:"target project id or shared path id"`
	Reason string `json:"reason" jsonschema_description:"evidenced dependency reason"`
}

type WorkspaceRouteOutput struct {
	PathPattern string   `json:"path_pattern" jsonschema_description:"path glob relative to workspace root"`
	ProjectIDs  []string `json:"project_ids" jsonschema_description:"affected project ids"`
	Reason      string   `json:"reason" jsonschema_description:"why this path affects those projects"`
}

type WorkspaceProfileOutput struct {
	Name         string                      `json:"name" jsonschema_description:"workspace name"`
	RootPath     string                      `json:"root_path" jsonschema_description:"workspace root path"`
	Summary      string                      `json:"summary,omitempty" jsonschema_description:"workspace-level factual summary"`
	Projects     []WorkspaceProjectOutput    `json:"projects" jsonschema_description:"child projects"`
	Shared       []WorkspacePathOutput       `json:"shared,omitempty" jsonschema_description:"shared paths"`
	Contracts    []WorkspacePathOutput       `json:"contracts,omitempty" jsonschema_description:"contract paths"`
	Infra        []WorkspacePathOutput       `json:"infra,omitempty" jsonschema_description:"infrastructure paths"`
	Dependencies []WorkspaceDependencyOutput `json:"dependencies,omitempty" jsonschema_description:"workspace dependencies"`
	ImpactRoutes []WorkspaceRouteOutput      `json:"impact_routes,omitempty" jsonschema_description:"impact routes"`
}

type WorkspaceRuleOutput struct {
	Title       string   `json:"title" jsonschema_description:"rule title"`
	Description string   `json:"description" jsonschema_description:"actionable workspace rule"`
	AppliesTo   []string `json:"applies_to,omitempty" jsonschema_description:"project ids, roles, or path families"`
}

type WorkspaceParallelGuidanceOutput struct {
	Scope     string `json:"scope" jsonschema_description:"project or path scope"`
	Allowed   bool   `json:"allowed" jsonschema_description:"true when concurrent work is allowed"`
	Condition string `json:"condition" jsonschema_description:"condition for safe parallelism"`
}

type WorkspaceLoadMultipleSkillOutput struct {
	Condition  string   `json:"condition" jsonschema_description:"when multiple child skills are needed"`
	ProjectIDs []string `json:"project_ids" jsonschema_description:"child project ids to load"`
	Reason     string   `json:"reason" jsonschema_description:"why these skills are needed together"`
}

type WorkspaceSpecOutput struct {
	Name                   string                             `json:"name" jsonschema_description:"workspace name"`
	RootPath               string                             `json:"root_path" jsonschema_description:"workspace root path"`
	Projects               []WorkspaceProjectOutput           `json:"projects" jsonschema_description:"child projects"`
	Routing                []WorkspaceRouteOutput             `json:"routing" jsonschema_description:"routing rules"`
	Rules                  []WorkspaceRuleOutput              `json:"rules" jsonschema_description:"workspace-level rules"`
	ChangeOrder            []string                           `json:"change_order,omitempty" jsonschema_description:"ordered string steps; include step numbers inside each string"`
	ParallelAgentGuidance  []WorkspaceParallelGuidanceOutput  `json:"parallel_agent_guidance,omitempty" jsonschema_description:"parallel agent boundaries"`
	LoadMultipleSkillsWhen []WorkspaceLoadMultipleSkillOutput `json:"load_multiple_skills_when,omitempty" jsonschema_description:"when to load multiple child skills"`
}

type OptimizeWorkflowOutput struct {
	Title       string   `json:"title" jsonschema_description:"short title"`
	Summary     string   `json:"summary" jsonschema_description:"one-sentence summary"`
	Content     string   `json:"content" jsonschema_description:"complete Markdown workflow body"`
	Suggestions []string `json:"suggestions" jsonschema_description:"gaps or conflicts"`
}
