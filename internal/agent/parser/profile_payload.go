package parser

import (
	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/domain"
)

func projectProfileToAnalyzeProjectResult(p aicontract.ProjectProfileOutput) *agent.AnalyzeProjectResult {
	result := &agent.AnalyzeProjectResult{
		ProjectName:       p.ProjectName,
		Language:          p.Language,
		Frameworks:        stringsOrEmpty(p.Frameworks),
		Architecture:      p.Architecture,
		Structure:         p.Structure,
		DependencyGraph:   p.DependencyGraph,
		DataFlow:          p.DataFlow,
		FrameworkPatterns: stringsOrEmpty(p.FrameworkPatterns),
		ConfigPatterns:    stringsOrEmpty(p.ConfigPatterns),
		Dependencies:      stringsOrEmpty(p.Dependencies),
		Summary:           p.Summary,
		Layers:            architectureLayersToDomain(p.Layers),
		KeyModules:        modulesToDomain(p.KeyModules),
	}
	return result
}

func authorityExtractionToResult(p aicontract.AuthorityExtractionOutput) *agent.ExtractAuthorityResult {
	return &agent.ExtractAuthorityResult{AuthoritySections: authoritySectionsToAgent(p.AuthoritySections)}
}

func authoritySectionsToAgent(sections []aicontract.AuthoritySectionOutput) []agent.AuthoritySectionResult {
	out := make([]agent.AuthoritySectionResult, len(sections))
	for i, section := range sections {
		out[i] = agent.AuthoritySectionResult{
			SectionID:    section.SectionID,
			Rules:        authorityRulesToDomain(section.Rules),
			NoRuleReason: section.NoRuleReason,
		}
	}
	return out
}

func authorityRulesToDomain(rules []aicontract.AuthorityRuleOutput) []domain.EngineeringRule {
	out := make([]domain.EngineeringRule, len(rules))
	for i, rule := range rules {
		out[i] = domain.EngineeringRule{
			Title:         rule.Title,
			Rule:          rule.Rule,
			AppliesTo:     stringsOrEmpty(rule.AppliesTo),
			CommandPolicy: rule.CommandPolicy,
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
