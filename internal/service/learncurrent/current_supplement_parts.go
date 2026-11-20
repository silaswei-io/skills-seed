package learncurrent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
)

// supplementUnit 保持一条知识的全部证据或一个完整调用入口不可分割。
type supplementUnit struct {
	patterns []domain.Pattern
	symbols  []string
}

// supplementParts 只在整文件证据超限时，按知识与符号边界动态组合分片。
func supplementParts(root string, index *codegraph.Index, gap semanticGap, patterns []domain.Pattern) ([]*agent.LearningSupplement, error) {
	var units []supplementUnit
	for _, pattern := range domain.ActivePatterns(patterns) {
		if pattern.AllowsHardConstraint() || !pattern.IsSemanticKnowledge() {
			continue
		}
		for _, evidence := range pattern.EvidenceLocations {
			if evidence.Path == gap.Path {
				units = append(units, supplementUnit{patterns: []domain.Pattern{pattern}})
				break
			}
		}
	}
	sort.Slice(units, func(i, j int) bool { return units[i].patterns[0].ID < units[j].patterns[0].ID })
	symbols := append([]string(nil), gap.Symbols...)
	if len(symbols) == 0 && !gap.Existing {
		for _, node := range index.Nodes {
			if node.Path == gap.Path && !node.DeclarationOnly && (node.Kind == "function" || node.Kind == "method" || node.Kind == "route") {
				symbols = append(symbols, node.ID)
			}
		}
	}
	sort.Strings(symbols)
	for _, id := range slices.Compact(symbols) {
		units = append(units, supplementUnit{symbols: []string{id}})
	}
	if len(units) == 0 {
		return nil, supplementCapacityError(fmt.Sprintf("source evidence %s exceeds safety limit without independent knowledge or callable units", gap.Path))
	}

	nodes := make(map[string]codegraph.Node, len(index.Nodes))
	outgoing := make(map[string][]string)
	for _, node := range index.Nodes {
		nodes[node.ID] = node
	}
	for _, call := range index.Calls {
		outgoing[call.Source] = append(outgoing[call.Source], call.Target)
	}
	build := func(selected []supplementUnit) (*agent.LearningSupplement, error) {
		part := semanticGap{Path: gap.Path, Reason: gap.Reason, Priority: gap.Priority}
		var related []domain.Pattern
		ids := make([]string, 0)
		for _, unit := range selected {
			related = append(related, unit.patterns...)
			for _, pattern := range unit.patterns {
				ids = append(ids, pattern.ID)
			}
			part.Symbols = append(part.Symbols, unit.symbols...)
			for _, id := range unit.symbols {
				part.Related = append(part.Related, callWitness(nodes[id], outgoing, nodes)...)
			}
		}
		part.Existing = len(related) > 0
		input, err := supplementEvidence(root, index, []semanticGap{part}, related)
		if err != nil {
			return nil, fmt.Errorf("source evidence %s (patterns %v, symbols %v): %w", gap.Path, ids, part.Symbols, err)
		}
		identity, _ := json.Marshal(struct {
			Path              string
			Patterns, Symbols []string
		}{gap.Path, ids, part.Symbols})
		input.Focus.ID = fmt.Sprintf("semantic-%x", sha256.Sum256(identity))[:25]
		input.Partial, input.PatternIDs = true, ids
		return input, nil
	}

	var parts []*agent.LearningSupplement
	var selected []supplementUnit
	var current *agent.LearningSupplement
	for _, unit := range units {
		next := append(append([]supplementUnit(nil), selected...), unit)
		input, err := build(next)
		if err != nil && len(selected) > 0 && errors.Is(err, errSupplementEvidenceCapacity) {
			parts = append(parts, current)
			selected = nil
			input, err = build([]supplementUnit{unit})
		}
		if err != nil {
			return nil, err
		}
		selected = append(selected, unit)
		current = input
	}
	return append(parts, current), nil
}

// commitSupplementParts 仅在全部分片完成且全部证据仍有效时提交文件基线。
func commitSupplementParts(ctx context.Context, cont *container.Container, root string, changes *fileanalysis.FileChanges, job supplementJob) error {
	for _, input := range job.parts {
		for path, expected := range input.SourceHashes {
			actual, err := sourcecode.EvidenceDigest(root, path)
			if err != nil {
				return fmt.Errorf("read source evidence %s before file completion: %w", path, err)
			}
			if actual != expected {
				return fmt.Errorf("source evidence %s changed before file completion: %w", path, agent.ErrSupplementUnresolved)
			}
		}
	}
	completed := &fileanalysis.FileChanges{Scope: changes.Scope}
	for _, record := range changes.Records {
		if slices.Contains(job.input.Focus.EntryPaths, record.Path) {
			record.AnalysisStatus = domain.FileAnalysisStatusAnalyzed
			record.SelectionReason = ""
			completed.Records = append(completed.Records, record)
		}
	}
	return fileanalysis.CommitCurrentChanges(ctx, cont.FileTracker, completed)
}
