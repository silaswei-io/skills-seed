package learn

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
	workspacediscovery "github.com/silaswei-io/skills-seed/internal/workspace"
)

type workspaceLearnInputData struct {
	Name       string                       `json:"name"`
	RootPath   string                       `json:"root_path"`
	Projects   []workspaceLearnInputProject `json:"projects"`
	ConfigPath string                       `json:"config_path,omitempty"`
}

type workspaceLearnInputProject struct {
	ID                 string   `json:"id"`
	Path               string   `json:"path"`
	Type               string   `json:"type"`
	Language           string   `json:"language"`
	SkillPath          string   `json:"skill_path,omitempty"`
	ProjectProfilePath string   `json:"project_profile_path,omitempty"`
	ProjectSpecPath    string   `json:"project_spec_path,omitempty"`
	Summary            string   `json:"summary,omitempty"`
	Frameworks         []string `json:"frameworks,omitempty"`
	KeyModules         []string `json:"key_modules,omitempty"`
}

type workspaceRelationshipFingerprintInput struct {
	Kind           string                  `json:"kind"`
	WorkspaceInput workspaceLearnInputData `json:"workspace_input"`
	UserContext    string                  `json:"user_context,omitempty"`
}

type workspaceRelationshipOptions struct {
	DirtyProjectIDs []string
	ProfileMode     string
}

func saveWorkspaceRelationshipArtifacts(ctx context.Context, cont *container.Container, workspaceName, projectRoot string, workspaceConfig config.WorkspaceConfig, opts workspaceRelationshipOptions) (bool, error) {
	if cont.WorkspaceProfileRepo == nil && cont.WorkspaceSpecRepo == nil {
		return false, nil
	}
	generatedAt := time.Now().Format(time.RFC3339)
	baseProfile := workspacediscovery.ProfileFromConfig(workspaceName, projectRoot, workspaceConfig)
	if cont.Agent == nil {
		return true, saveWorkspaceRelationshipFallback(ctx, cont, baseProfile, generatedAt)
	}

	input, err := workspaceLearnInput(ctx, cont, workspaceName, projectRoot, workspaceConfig)
	if err != nil {
		return false, err
	}
	userContext := runtimecontext.UserContext(ctx)
	decision, err := domain.PrepareInputFingerprint(ctx, cont.FileTracker, workspaceRelationshipFingerprintScope(), "workspace-relationships.json", workspaceRelationshipFingerprintInput{
		Kind:           "workspace_relationship_learning",
		WorkspaceInput: input,
		UserContext:    userContext,
	})
	if err != nil {
		return false, err
	}
	if workspaceRelationshipShouldSkip(ctx, cont, input, decision, opts, userContext) {
		if err := decision.Commit(ctx, cont.FileTracker); err != nil {
			return false, err
		}
		logger.Info(i18n.Get("LearnWorkspaceRelationshipsSkipped"))
		return false, nil
	}
	runtimeDir := filepath.Join(projectRoot, ".skills-seed", "memory", "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return false, err
	}
	tmpDir, err := os.MkdirTemp(runtimeDir, runtimefiles.TempPattern("workspace-learn"))
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmpDir)

	inputPath, err := writeJSONInput(filepath.Join(tmpDir, "workspace-input.json"), input)
	if err != nil {
		return false, err
	}
	userContextPath := ""
	if userContext != "" {
		userContextPath = filepath.Join(tmpDir, "user-context.md")
		if err := os.WriteFile(userContextPath, []byte(userContext+"\n"), 0600); err != nil {
			return false, err
		}
	}

	tracker := progress.New(3)
	var profile *domain.WorkspaceProfile
	if err := tracker.RunStep(i18n.Get("ProgressLearnWorkspaceAnalyzeProfile"), func() error {
		var err error
		profile, err = cont.Agent.AnalyzeWorkspaceProfile(ctx, &agent.AnalyzeWorkspaceProfileRequest{
			WorkspaceName:      workspaceName,
			WorkspaceRoot:      projectRoot,
			WorkspaceInputPath: inputPath,
			UserContextPath:    userContextPath,
		})
		if err != nil {
			return err
		}
		profile = workspacediscovery.MergeProfile(baseProfile, profile)
		profile.GeneratedAt = generatedAt
		return nil
	}); err != nil {
		return false, err
	}

	var spec *domain.WorkspaceSpec
	if err := tracker.RunStep(i18n.Get("ProgressLearnWorkspaceAnalyzeSpec"), func() error {
		profilePath, err := writeJSONInput(filepath.Join(tmpDir, "workspace-profile.json"), profile)
		if err != nil {
			return err
		}
		spec, err = cont.Agent.AnalyzeWorkspaceSpec(ctx, &agent.AnalyzeWorkspaceSpecRequest{
			WorkspaceName:        workspaceName,
			WorkspaceRoot:        projectRoot,
			WorkspaceInputPath:   inputPath,
			WorkspaceProfilePath: profilePath,
			UserContextPath:      userContextPath,
		})
		if err != nil {
			return err
		}
		spec = workspacediscovery.MergeSpec(workspacediscovery.SpecFromProfile(profile), spec)
		spec.GeneratedAt = generatedAt
		return nil
	}); err != nil {
		return false, err
	}

	if err := tracker.RunStep(i18n.Get("ProgressLearnWorkspaceSaveArtifacts"), func() error {
		if cont.WorkspaceProfileRepo != nil {
			if err := cont.WorkspaceProfileRepo.Save(ctx, profile); err != nil {
				return err
			}
		}
		if cont.WorkspaceSpecRepo != nil {
			if err := cont.WorkspaceSpecRepo.Save(ctx, spec); err != nil {
				return err
			}
		}
		if err := decision.Commit(ctx, cont.FileTracker); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return false, err
	}
	return true, nil
}

func workspaceRelationshipFingerprintScope() domain.FileAnalysisScope {
	return domain.FileAnalysisScope{ProjectID: "__workspace__", ScopePath: "learn"}
}

func workspaceRelationshipShouldSkip(ctx context.Context, cont *container.Container, input workspaceLearnInputData, decision *domain.InputFingerprintDecision, opts workspaceRelationshipOptions, userContext string) bool {
	if !workspaceRelationshipArtifactsExist(ctx, cont) {
		return false
	}
	if decision.ShouldSkip() {
		return true
	}
	if opts.ProfileMode == learnCurrentProfileRefresh || userContext != "" || len(opts.DirtyProjectIDs) > 0 {
		return false
	}
	return workspaceRelationshipArtifactsMatchInput(ctx, cont, input)
}

func workspaceRelationshipArtifactsExist(ctx context.Context, cont *container.Container) bool {
	if cont.WorkspaceProfileRepo != nil {
		if _, err := cont.WorkspaceProfileRepo.Get(ctx); err != nil {
			return false
		}
	}
	if cont.WorkspaceSpecRepo != nil {
		if _, err := cont.WorkspaceSpecRepo.Get(ctx); err != nil {
			return false
		}
	}
	return true
}

func workspaceRelationshipArtifactsMatchInput(ctx context.Context, cont *container.Container, input workspaceLearnInputData) bool {
	if cont.WorkspaceProfileRepo != nil {
		profile, err := cont.WorkspaceProfileRepo.Get(ctx)
		if err != nil || !workspaceProfileMatchesInput(profile, input) {
			return false
		}
	}
	if cont.WorkspaceSpecRepo != nil {
		spec, err := cont.WorkspaceSpecRepo.Get(ctx)
		if err != nil || !workspaceSpecMatchesInput(spec, input) {
			return false
		}
	}
	return true
}

func workspaceProfileMatchesInput(profile *domain.WorkspaceProfile, input workspaceLearnInputData) bool {
	if profile == nil || profile.Name != input.Name || profile.RootPath != input.RootPath {
		return false
	}
	return workspaceProjectsMatchInput(profile.Projects, input.Projects)
}

func workspaceSpecMatchesInput(spec *domain.WorkspaceSpec, input workspaceLearnInputData) bool {
	if spec == nil || spec.Name != input.Name || spec.RootPath != input.RootPath {
		return false
	}
	return workspaceProjectsMatchInput(spec.Projects, input.Projects)
}

func workspaceProjectsMatchInput(projects []domain.WorkspaceProject, inputProjects []workspaceLearnInputProject) bool {
	if len(projects) != len(inputProjects) {
		return false
	}
	for i, project := range projects {
		inputProject := inputProjects[i]
		if project.ID != inputProject.ID ||
			project.Path != inputProject.Path ||
			project.Type != inputProject.Type ||
			project.Language != inputProject.Language {
			return false
		}
	}
	return true
}

func saveWorkspaceRelationshipFallback(ctx context.Context, cont *container.Container, profile *domain.WorkspaceProfile, generatedAt string) error {
	if profile == nil {
		profile = &domain.WorkspaceProfile{}
	}
	profile.GeneratedAt = generatedAt
	if cont.WorkspaceProfileRepo != nil {
		if err := cont.WorkspaceProfileRepo.Save(ctx, profile); err != nil {
			return err
		}
	}
	if cont.WorkspaceSpecRepo != nil {
		spec := workspacediscovery.SpecFromProfile(profile)
		spec.GeneratedAt = generatedAt
		if err := cont.WorkspaceSpecRepo.Save(ctx, spec); err != nil {
			return err
		}
	}
	return nil
}

func workspaceLearnInput(ctx context.Context, cont *container.Container, workspaceName, projectRoot string, workspaceConfig config.WorkspaceConfig) (workspaceLearnInputData, error) {
	input := workspaceLearnInputData{
		Name:       workspaceName,
		RootPath:   projectRoot,
		Projects:   make([]workspaceLearnInputProject, 0, len(workspaceConfig.Projects)),
		ConfigPath: filepath.ToSlash(filepath.Join(projectRoot, ".skills-seed", "config.yaml")),
	}
	for _, project := range workspaceConfig.Projects {
		projectRootPath, err := workspacediscovery.ResolveProjectRoot(projectRoot, project)
		if err != nil {
			return workspaceLearnInputData{}, err
		}
		childSeedPath := filepath.Join(projectRootPath, ".skills-seed")
		projectProfilePath := filepath.Join(childSeedPath, "memory", "project-profile.json")
		projectSpecPath := filepath.Join(childSeedPath, "memory", "project-spec.json")
		skillPath := workspaceChildSkillPath(projectRootPath, childSeedPath, cont.ConfigRepo)
		child := workspaceLearnInputProject{
			ID:                 project.ID,
			Path:               project.Path,
			Type:               project.Type,
			Language:           project.Language,
			SkillPath:          filepath.ToSlash(filepath.Join(project.Path, skillPath, "SKILL.md")),
			ProjectProfilePath: filepath.ToSlash(projectProfilePath),
			ProjectSpecPath:    filepath.ToSlash(projectSpecPath),
		}
		if profile, err := readChildProjectProfile(ctx, cont, project.ID, projectProfilePath); err == nil && profile != nil {
			child.Summary = profile.Summary
			child.Frameworks = append([]string(nil), profile.Frameworks...)
			child.KeyModules = projectProfileModuleSummaries(profile)
		}
		input.Projects = append(input.Projects, child)
	}
	return input, nil
}

func workspaceChildSkillPath(projectRootPath, childSeedPath string, rootConfig config.Reader) string {
	configRepo, err := config.NewRepository(childSeedPath, "")
	if err == nil {
		target := configRepo.GetEffectiveSkillsTarget()
		outputPath := configRepo.GetEffectiveSkillsPath()
		if outputPath == "" {
			outputPath = config.DefaultSkillsPathForTarget(target)
		}
		return filepath.ToSlash(outputPath)
	}

	target := ""
	outputPath := ""
	if rootConfig != nil {
		target = rootConfig.GetEffectiveSkillsTarget()
		outputPath = rootConfig.GetEffectiveSkillsPath()
	}
	if outputPath == "" {
		outputPath = config.DefaultSkillsPathForTarget(target)
	}
	return filepath.ToSlash(outputPath)
}

func readChildProjectProfile(ctx context.Context, cont *container.Container, projectID, profilePath string) (*domain.ProjectProfile, error) {
	if cont.ProfileRepo != nil {
		if profile, err := cont.ProfileRepo.GetForProject(ctx, projectID); err == nil {
			return profile, nil
		}
	}
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, err
	}
	var profile domain.ProjectProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func projectProfileModuleSummaries(profile *domain.ProjectProfile) []string {
	if profile == nil {
		return nil
	}
	modules := make([]string, 0, len(profile.KeyModules))
	for _, module := range profile.KeyModules {
		if module.Path == "" && module.Description == "" {
			continue
		}
		if module.Description == "" {
			modules = append(modules, module.Path)
			continue
		}
		if module.Path == "" {
			modules = append(modules, module.Description)
			continue
		}
		modules = append(modules, module.Path+": "+module.Description)
	}
	return modules
}

func writeJSONInput(path string, value interface{}) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}
