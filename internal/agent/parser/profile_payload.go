package parser

import (
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

func projectProfileToAnalyzeProjectResult(p aicontract.ProjectProfileOutput, now time.Time) *agent.AnalyzeProjectResult {
	result := &agent.AnalyzeProjectResult{
		ProjectName:        p.ProjectName,
		Language:           p.Language,
		Frameworks:         stringsOrEmpty(p.Frameworks),
		Architecture:       p.Architecture,
		Structure:          p.Structure,
		DependencyGraph:    p.DependencyGraph,
		DataFlow:           p.DataFlow,
		FrameworkPatterns:  stringsOrEmpty(p.FrameworkPatterns),
		ConfigPatterns:     stringsOrEmpty(p.ConfigPatterns),
		Dependencies:       stringsOrEmpty(p.Dependencies),
		EngineeringRules:   engineeringRulesToDomain(p.EngineeringRules),
		ValidationCommands: validationCommandsToDomain(p.ValidationCommands),
		Summary:            p.Summary,
		Layers:             architectureLayersToDomain(p.Layers),
		CommonUtils:        utilityFunctionsToDomain(p.CommonUtils),
		KeyModules:         modulesToDomain(p.KeyModules),
	}

	result.BusinessMethods = businessMethodsToDomain(p.BusinessMethods, now)
	return result
}

func engineeringRulesToDomain(rules []aicontract.EngineeringRuleOutput) []domain.EngineeringRule {
	out := make([]domain.EngineeringRule, len(rules))
	for i, rule := range rules {
		out[i] = domain.EngineeringRule{
			Title:    rule.Title,
			Rule:     rule.Rule,
			Source:   rule.Source,
			Evidence: stringsOrEmpty(rule.Evidence),
		}
	}
	return out
}

func architectureLayersToDomain(layers []aicontract.ArchitectureLayerOutput) []domain.ArchitectureLayer {
	out := make([]domain.ArchitectureLayer, len(layers))
	for i, layer := range layers {
		out[i] = domain.ArchitectureLayer{
			Name:             layer.Name,
			Description:      layer.Description,
			Responsibilities: stringsOrEmpty(layer.Responsibilities),
			Files:            stringsOrEmpty(layer.Files),
		}
	}
	return out
}

func utilityFunctionsToDomain(utils []aicontract.UtilityFunctionOutput) []domain.UtilityFunction {
	out := make([]domain.UtilityFunction, len(utils))
	for i, util := range utils {
		out[i] = domain.UtilityFunction{
			Name:        util.Name,
			File:        util.File,
			Signature:   util.Signature,
			Description: util.Description,
			Usage:       util.Usage,
		}
	}
	return out
}

func modulesToDomain(modules []aicontract.ModuleOutput) []domain.ModuleInfo {
	out := make([]domain.ModuleInfo, len(modules))
	for i, module := range modules {
		out[i] = domain.ModuleInfo{
			Name:             module.Name,
			Path:             module.Path,
			Description:      module.Description,
			Responsibilities: stringsOrEmpty(module.Responsibilities),
			Dependencies:     stringsOrEmpty(module.Dependencies),
			Dependents:       stringsOrEmpty(module.Dependents),
			KeyMethods:       stringsOrEmpty(module.KeyMethods),
		}
	}
	return out
}

func validationCommandsToDomain(commands []aicontract.ValidationCommandOutput) []domain.ValidationCommand {
	out := make([]domain.ValidationCommand, len(commands))
	for i, command := range commands {
		out[i] = domain.ValidationCommand{
			Command:    command.Command,
			When:       command.When,
			Source:     command.Source,
			Workdir:    command.Workdir,
			ScopePaths: stringsOrEmpty(command.ScopePaths),
			Evidence:   stringsOrEmpty(command.Evidence),
			Type:       command.Type,
		}
	}
	return out
}
