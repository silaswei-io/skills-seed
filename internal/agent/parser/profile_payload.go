package parser

import (
	"encoding/json"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

type stringList []string

func (l *stringList) UnmarshalJSON(data []byte) error {
	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*l = values
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		if value == "" {
			*l = []string{}
		} else {
			*l = []string{value}
		}
		return nil
	}
	return json.Unmarshal(data, &values)
}

func (l stringList) strings() []string {
	return nonNilStrings([]string(l))
}

type projectProfilePayload struct {
	ProjectName  string     `json:"project_name"`
	Language     string     `json:"language"`
	Frameworks   stringList `json:"frameworks"`
	Architecture string     `json:"architecture"`
	Structure    string     `json:"structure"`
	Layers       []struct {
		Name             string     `json:"name"`
		Description      string     `json:"description"`
		Responsibilities stringList `json:"responsibilities"`
		Files            stringList `json:"files"`
	} `json:"layers"`
	DependencyGraph   string     `json:"dependency_graph"`
	DataFlow          string     `json:"data_flow"`
	FrameworkPatterns stringList `json:"framework_patterns"`
	CommonUtils       []struct {
		Name        string `json:"name"`
		File        string `json:"file"`
		Signature   string `json:"signature"`
		Description string `json:"description"`
		Usage       string `json:"usage"`
	} `json:"common_utils"`
	KeyModules []struct {
		Name             string     `json:"name"`
		Path             string     `json:"path"`
		Description      string     `json:"description"`
		Responsibilities stringList `json:"responsibilities"`
		Dependencies     stringList `json:"dependencies"`
		Dependents       stringList `json:"dependents"`
		KeyMethods       stringList `json:"key_methods"`
	} `json:"key_modules"`
	ConfigPatterns     stringList                 `json:"config_patterns"`
	Dependencies       stringList                 `json:"dependencies"`
	BusinessMethods    []businessMethodPayload    `json:"business_methods"`
	ValidationCommands []domain.ValidationCommand `json:"validation_commands"`
	Summary            string                     `json:"summary"`
}

func (p projectProfilePayload) toAnalyzeProjectResult(now time.Time) *agent.AnalyzeProjectResult {
	result := &agent.AnalyzeProjectResult{
		ProjectName:        p.ProjectName,
		Language:           p.Language,
		Frameworks:         p.Frameworks.strings(),
		Architecture:       p.Architecture,
		Structure:          p.Structure,
		DependencyGraph:    p.DependencyGraph,
		DataFlow:           p.DataFlow,
		FrameworkPatterns:  p.FrameworkPatterns.strings(),
		ConfigPatterns:     p.ConfigPatterns.strings(),
		Dependencies:       p.Dependencies.strings(),
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
			Responsibilities: layer.Responsibilities.strings(),
			Files:            layer.Files.strings(),
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
			Responsibilities: module.Responsibilities.strings(),
			Dependencies:     module.Dependencies.strings(),
			Dependents:       module.Dependents.strings(),
			KeyMethods:       module.KeyMethods.strings(),
		}
	}

	result.BusinessMethods = businessMethodsToDomain(p.BusinessMethods, now)
	return result
}

func (p projectProfilePayload) toProjectProfileDelta(now time.Time) domain.ProjectProfileDelta {
	projectResult := p.toAnalyzeProjectResult(now)
	return domain.ProjectProfileDelta{
		Frameworks:         p.Frameworks.strings(),
		Dependencies:       p.Dependencies.strings(),
		Layers:             projectResult.Layers,
		KeyModules:         projectResult.KeyModules,
		CommonUtils:        projectResult.CommonUtils,
		ConfigPatterns:     p.ConfigPatterns.strings(),
		FrameworkPatterns:  p.FrameworkPatterns.strings(),
		BusinessMethods:    projectResult.BusinessMethods,
		ValidationCommands: p.ValidationCommands,
		Summary:            p.Summary,
		Architecture:       p.Architecture,
		Structure:          p.Structure,
		DependencyGraph:    p.DependencyGraph,
		DataFlow:           p.DataFlow,
	}
}
