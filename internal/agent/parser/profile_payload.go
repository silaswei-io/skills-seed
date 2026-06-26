package parser

import (
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

type projectProfilePayload struct {
	ProjectName  string   `json:"project_name"`
	Language     string   `json:"language"`
	Frameworks   []string `json:"frameworks"`
	Architecture string   `json:"architecture"`
	Structure    string   `json:"structure"`
	Layers       []struct {
		Name             string   `json:"name"`
		Description      string   `json:"description"`
		Responsibilities []string `json:"responsibilities"`
		Files            []string `json:"files"`
	} `json:"layers"`
	DependencyGraph   string   `json:"dependency_graph"`
	DataFlow          string   `json:"data_flow"`
	FrameworkPatterns []string `json:"framework_patterns"`
	CommonUtils       []struct {
		Name        string `json:"name"`
		File        string `json:"file"`
		Signature   string `json:"signature"`
		Description string `json:"description"`
		Usage       string `json:"usage"`
	} `json:"common_utils"`
	KeyModules []struct {
		Name             string   `json:"name"`
		Path             string   `json:"path"`
		Description      string   `json:"description"`
		Responsibilities []string `json:"responsibilities"`
		Dependencies     []string `json:"dependencies"`
		Dependents       []string `json:"dependents"`
		KeyMethods       []string `json:"key_methods"`
	} `json:"key_modules"`
	ConfigPatterns     []string                   `json:"config_patterns"`
	Dependencies       []string                   `json:"dependencies"`
	BusinessMethods    []businessMethodPayload    `json:"business_methods"`
	ValidationCommands []domain.ValidationCommand `json:"validation_commands"`
	Summary            string                     `json:"summary"`
}

func (p projectProfilePayload) toAnalyzeProjectResult(now time.Time) *agent.AnalyzeProjectResult {
	result := &agent.AnalyzeProjectResult{
		ProjectName:        p.ProjectName,
		Language:           p.Language,
		Frameworks:         nonNilStrings(p.Frameworks),
		Architecture:       p.Architecture,
		Structure:          p.Structure,
		DependencyGraph:    p.DependencyGraph,
		DataFlow:           p.DataFlow,
		FrameworkPatterns:  p.FrameworkPatterns,
		ConfigPatterns:     nonNilStrings(p.ConfigPatterns),
		Dependencies:       nonNilStrings(p.Dependencies),
		ValidationCommands: p.ValidationCommands,
		Summary:            p.Summary,
		Layers:             make([]domain.ArchitectureLayer, len(p.Layers)),
		CommonUtils:        make([]domain.UtilityFunction, len(p.CommonUtils)),
		KeyModules:         make([]domain.ModuleInfo, len(p.KeyModules)),
	}

	for i, layer := range p.Layers {
		result.Layers[i] = domain.ArchitectureLayer{
			Name:             layer.Name,
			Description:      layer.Description,
			Responsibilities: layer.Responsibilities,
			Files:            layer.Files,
		}
	}

	for i, util := range p.CommonUtils {
		result.CommonUtils[i] = domain.UtilityFunction{
			Name:        util.Name,
			File:        util.File,
			Signature:   util.Signature,
			Description: util.Description,
			Usage:       util.Usage,
		}
	}

	for i, module := range p.KeyModules {
		result.KeyModules[i] = domain.ModuleInfo{
			Name:             module.Name,
			Path:             module.Path,
			Description:      module.Description,
			Responsibilities: module.Responsibilities,
			Dependencies:     module.Dependencies,
			Dependents:       module.Dependents,
			KeyMethods:       module.KeyMethods,
		}
	}

	result.BusinessMethods = businessMethodsToDomain(p.BusinessMethods, now)
	return result
}

func (p projectProfilePayload) toProjectProfileDelta(now time.Time) domain.ProjectProfileDelta {
	projectResult := p.toAnalyzeProjectResult(now)
	return domain.ProjectProfileDelta{
		Frameworks:         p.Frameworks,
		Dependencies:       p.Dependencies,
		Layers:             projectResult.Layers,
		KeyModules:         projectResult.KeyModules,
		CommonUtils:        projectResult.CommonUtils,
		ConfigPatterns:     p.ConfigPatterns,
		FrameworkPatterns:  p.FrameworkPatterns,
		BusinessMethods:    projectResult.BusinessMethods,
		ValidationCommands: p.ValidationCommands,
		Summary:            p.Summary,
		Architecture:       p.Architecture,
		Structure:          p.Structure,
		DependencyGraph:    p.DependencyGraph,
		DataFlow:           p.DataFlow,
	}
}
