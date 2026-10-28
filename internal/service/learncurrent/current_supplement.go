package learncurrent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/codegraph"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
	"github.com/silaswei-io/skills-seed/internal/service/fileanalysis"
	"github.com/silaswei-io/skills-seed/internal/sourcecode"
	"github.com/silaswei-io/skills-seed/internal/terminal/logger"
)

const (
	// supplementSourceLimit 控制普通批次大小，为分析结果和独立审查保留空间。
	supplementSourceLimit = 64 * 1024
	// supplementSingleSourceLimit 为单文件证据保留结构化请求和提示词空间。
	supplementSingleSourceLimit = agent.SupplementInputLimit - 32*1024
	// supplementEvidenceRevision 在证据选择策略变化后使旧的容量阻塞记录失效。
	supplementEvidenceRevision = 1
)

var errSupplementEvidenceCapacity = errors.New("supplement evidence exceeds safe capacity")

type supplementCapacityError string

func (e supplementCapacityError) Error() string { return string(e) }

func (e supplementCapacityError) Is(target error) bool {
	return target == errSupplementEvidenceCapacity || target == agent.ErrSupplementUnresolved
}

type semanticGap struct {
	Path     string   `json:"path"`
	Reason   string   `json:"reason"`
	Priority int      `json:"priority"`
	Symbols  []string `json:"-"`
	Related  []string `json:"-"`
	Existing bool     `json:"-"`
}

type supplementStatus struct {
	IndexedFiles       int                        `json:"indexed_files"`
	UnindexedFiles     int                        `json:"unindexed_files"`
	ChangedFiles       int                        `json:"changed_files"`
	SemanticCandidates int                        `json:"semantic_candidates"`
	ScreenedOut        int                        `json:"screened_out,omitempty"`
	Screened           map[string]string          `json:"screened,omitempty"`
	Blocked            map[string]supplementBlock `json:"blocked,omitempty"`
	Pending            []semanticGap              `json:"pending"`
	LastError          string                     `json:"last_error,omitempty"`
}

type supplementBlock struct {
	SourceHash string `json:"source_hash"`
	InputHash  string `json:"input_hash"`
	Reason     string `json:"reason"`
	Revision   int    `json:"revision"`
}

func supplementInputHash(index *codegraph.Index, gap semanticGap, patterns []domain.Pattern) string {
	ids := append(append([]string(nil), gap.Symbols...), gap.Related...)
	sort.Strings(ids)
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	nodes := make(map[string]codegraph.Node, len(ids))
	for _, node := range index.Nodes {
		if wanted[node.ID] {
			nodes[node.ID] = node
		}
	}
	type symbol struct {
		ID        string
		Path      string
		Signature string
		Line      int
		EndLine   int
	}
	selected := make([]symbol, 0, len(ids))
	for _, id := range ids {
		node := nodes[id]
		selected = append(selected, symbol{ID: id, Path: node.Path, Signature: node.Signature, Line: node.Line, EndLine: node.EndLine})
	}
	related := make([]domain.Pattern, 0)
	for _, pattern := range domain.ActivePatterns(patterns) {
		if pattern.AllowsHardConstraint() || !pattern.IsSemanticKnowledge() {
			continue
		}
		for _, evidence := range pattern.EvidenceLocations {
			if evidence.Path == gap.Path {
				related = append(related, pattern)
				break
			}
		}
	}
	sort.Slice(related, func(i, j int) bool { return related[i].ID < related[j].ID })
	data, _ := json.Marshal(struct {
		Symbols  []symbol
		Patterns []domain.Pattern
	}{selected, related})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// selectSemanticGaps 选择图无法解释的行为边界与公开能力；后续增量复核调用实现。
func selectSemanticGaps(index *codegraph.Index, changed map[string]bool, patterns []domain.Pattern, initial bool) []semanticGap {
	gaps := make(map[string]semanticGap)
	nodes := make(map[string]codegraph.Node, len(index.Nodes))
	for _, node := range index.Nodes {
		nodes[node.ID] = node
	}
	incoming := make(map[string]bool)
	crossFileIncoming := make(map[string]bool)
	outgoing := make(map[string][]string)
	for _, call := range index.Calls {
		source, sourceOK := nodes[call.Source]
		if !sourceOK {
			continue
		}
		target, targetOK := nodes[call.Target]
		if !targetOK {
			continue
		}
		incoming[call.Target] = true
		if source.Path != target.Path {
			crossFileIncoming[call.Target] = true
		}
		outgoing[call.Source] = append(outgoing[call.Source], call.Target)
	}
	for _, node := range index.Nodes {
		if !changed[node.Path] || node.DeclarationOnly || sourcecode.IsProcedureSource(node.Path) {
			continue
		}
		if node.Kind != "function" && node.Kind != "method" && node.Kind != "route" {
			continue
		}
		witness := callWitness(node, outgoing, nodes)
		crossFileCall := false
		for _, id := range witness {
			crossFileCall = crossFileCall || nodes[id].Path != node.Path
		}
		if initial && incoming[node.ID] && node.Kind != "route" && !node.Exported {
			continue
		}
		// 公开入口即使没有已知调用方，也可能是外部复用能力。
		if !crossFileCall && node.Kind != "route" && (initial || !crossFileIncoming[node.ID]) && !node.Exported {
			continue
		}
		gap := gaps[node.Path]
		gap.Path = node.Path
		gap.Reason = "changed callable: determine source-backed responsibility, reusable capability or contract beyond the local graph"
		gap.Priority += len(witness) + 1
		gap.Symbols = append(gap.Symbols, node.ID)
		gap.Related = append(gap.Related, witness...)
		gaps[node.Path] = gap
	}
	for _, pattern := range domain.ActivePatterns(patterns) {
		if pattern.AllowsHardConstraint() || !pattern.IsSemanticKnowledge() {
			continue
		}
		for _, evidence := range pattern.EvidenceLocations {
			if changed[evidence.Path] {
				gap := gaps[evidence.Path]
				gap.Path = evidence.Path
				gap.Reason = "accepted knowledge has changed source evidence; verify, revise or retire its semantic claims"
				gap.Priority = 1000000
				gap.Existing = true
				gaps[evidence.Path] = gap
			}
		}
	}
	result := make([]semanticGap, 0, len(gaps))
	for _, gap := range gaps {
		result = append(result, gap)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].Path < result[j].Path
	})
	// 从可确认的入口沿真实调用边覆盖下游；调用环没有可确认根时保留候选。
	candidate := make(map[string]bool, len(result))
	entry := make(map[string]bool, len(result))
	for _, gap := range result {
		candidate[gap.Path] = true
		entry[gap.Path] = gap.Existing
		for _, id := range gap.Symbols {
			entry[gap.Path] = entry[gap.Path] || nodes[id].Exported || nodes[id].Kind == "route"
		}
	}
	callers := make(map[string][]string)
	for _, call := range index.Calls {
		callers[call.Target] = append(callers[call.Target], call.Source)
	}
	roots := make(map[string]bool)
	for _, gap := range result {
		for _, id := range gap.Symbols {
			upstream := false
			for _, sourceID := range callers[id] {
				source := nodes[sourceID]
				if source.Path != gap.Path && candidate[source.Path] && entry[source.Path] {
					upstream = true
					break
				}
			}
			if !upstream || nodes[id].Kind == "route" {
				roots[id] = true
			}
		}
	}
	covered := make(map[string]bool)
	queue := make([]string, 0, len(roots))
	for id := range roots {
		queue = append(queue, id)
		covered[id] = true
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, targetID := range outgoing[id] {
			if covered[targetID] {
				continue
			}
			target, targetOK := nodes[targetID]
			if !targetOK || (target.Path != nodes[id].Path && !candidate[target.Path]) {
				continue
			}
			covered[targetID] = true
			queue = append(queue, targetID)
		}
	}
	selected := result[:0]
	for _, gap := range result {
		if gap.Existing || len(gap.Symbols) == 0 {
			selected = append(selected, gap)
			continue
		}
		allCovered := true
		for _, id := range gap.Symbols {
			if roots[id] || !covered[id] {
				allCovered = false
				break
			}
		}
		if !allCovered {
			selected = append(selected, gap)
		}
	}
	return selected
}

// callWitness 收集入口在本文件的调用实现及跨文件边界，不遍历下游文件的依赖子图。
func callWitness(root codegraph.Node, outgoing map[string][]string, nodes map[string]codegraph.Node) []string {
	queue := []string{root.ID}
	parents := map[string]string{root.ID: ""}
	witness := make(map[string]bool)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, targetID := range outgoing[current] {
			if _, seen := parents[targetID]; seen {
				continue
			}
			target, ok := nodes[targetID]
			if !ok {
				continue
			}
			parents[targetID] = current
			if target.Path == root.Path {
				witness[targetID] = true
				queue = append(queue, targetID)
				continue
			}
			for id := targetID; id != root.ID; id = parents[id] {
				witness[id] = true
			}
		}
	}
	result := make([]string, 0, len(witness))
	for id := range witness {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// supplementEvidence 只发送问题涉及的完整符号及其直接调用目标。
func supplementEvidence(root string, index *codegraph.Index, gaps []semanticGap, patterns []domain.Pattern) (*agent.LearningSupplement, error) {
	sourceLimit := supplementSourceLimit
	if len(gaps) == 1 {
		sourceLimit = supplementSingleSourceLimit
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool)
	for _, file := range index.Files {
		allowed[file.Path] = true
	}
	for _, path := range index.Unindexed {
		allowed[path] = true
	}
	nodes := make(map[string]codegraph.Node, len(index.Nodes))
	for _, node := range index.Nodes {
		nodes[node.ID] = node
	}
	owned := make(map[string]bool)
	required := make(map[string]bool)
	full := make(map[string]bool)
	symbols := make(map[string][]string)
	for _, gap := range gaps {
		owned[gap.Path], required[gap.Path] = true, true
		full[gap.Path] = gap.Existing || len(gap.Symbols) == 0
		symbols[gap.Path] = append(symbols[gap.Path], gap.Symbols...)
	}
	anchors := make(map[string][]int)
	for _, pattern := range domain.ActivePatterns(patterns) {
		if pattern.AllowsHardConstraint() || !pattern.IsSemanticKnowledge() {
			continue
		}
		touches := false
		for _, evidence := range pattern.EvidenceLocations {
			touches = touches || owned[evidence.Path]
		}
		if touches {
			for _, evidence := range pattern.EvidenceLocations {
				if allowed[evidence.Path] {
					required[evidence.Path] = true
					full[evidence.Path] = true
					if evidence.Line > 0 {
						anchors[evidence.Path] = append(anchors[evidence.Path], evidence.Line)
					}
				}
			}
		}
	}
	related := make(map[string]bool)
	for _, gap := range gaps {
		for _, id := range gap.Related {
			node, ok := nodes[id]
			if !ok || !allowed[node.Path] || node.DeclarationOnly {
				continue
			}
			symbols[node.Path] = append(symbols[node.Path], id)
			if !required[node.Path] {
				related[node.Path] = true
			}
		}
	}
	sources := make(map[string]string)
	hashes := make(map[string]string)
	ranges := make(map[string][]agent.SourceRange)
	read := func(path string) error {
		if !allowed[path] || !filepath.IsLocal(path) {
			return fmt.Errorf("unsafe evidence path: %s", path)
		}
		resolved, err := filepath.EvalSymlinks(filepath.Join(canonicalRoot, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(canonicalRoot, resolved)
		if err != nil || !filepath.IsLocal(rel) {
			return fmt.Errorf("evidence leaves project: %s", path)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return err
		}
		// 巨型文件属于异常输入；正常文件的模型证据由符号范围控制。
		if info.Size() > 8*1024*1024 {
			return supplementCapacityError(fmt.Sprintf("source evidence %s exceeds 8 MiB safety limit", path))
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return err
		}
		excerpt, sourceRanges, err := supplementSource(path, data, symbols[path], anchors[path], full[path], sourceLimit, nodes)
		if err != nil {
			return err
		}
		if len(excerpt) > sourceLimit {
			return supplementCapacityError(fmt.Sprintf("source evidence %s: %d bytes exceeds %d-byte safety limit", path, len(excerpt), sourceLimit))
		}
		sources[path] = excerpt
		hashes[path] = fmt.Sprintf("%x", sha256.Sum256(data))
		ranges[path] = sourceRanges
		return nil
	}
	var paths []string
	for path := range required {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := read(path); err != nil {
			return nil, err
		}
	}
	neighbors := related
	for _, call := range index.Calls {
		source, sourceOK := nodes[call.Source]
		target, targetOK := nodes[call.Target]
		if !sourceOK || !targetOK || !owned[source.Path] || !allowed[target.Path] || required[target.Path] {
			continue
		}
		for _, id := range symbols[source.Path] {
			if id == call.Source {
				neighbors[target.Path] = true
				symbols[target.Path] = append(symbols[target.Path], call.Target)
				break
			}
		}
	}
	encode := func() ([]byte, error) {
		var calls []string
		for _, call := range index.Calls {
			source, sourceOK := nodes[call.Source]
			target, targetOK := nodes[call.Target]
			if !sourceOK || !targetOK || source.Path == target.Path {
				continue
			}
			if _, ok := sources[source.Path]; ok {
				if _, ok := sources[target.Path]; ok {
					calls = append(calls, source.Path+"::"+source.Name+" -> "+target.Path+"::"+target.Name)
				}
			}
		}
		return json.Marshal(struct {
			Questions []semanticGap     `json:"questions"`
			Sources   map[string]string `json:"sources"`
			Calls     []string          `json:"calls"`
			Boundary  string            `json:"boundary"`
		}{gaps, sources, calls, "Sources contain complete selected symbols or anchored evidence, not necessarily whole files. Missing dynamic targets, external services and omitted neighbors remain unknown. Batch membership does not establish a business domain."})
	}
	data, err := encode()
	if err != nil {
		return nil, err
	}
	if len(data) > sourceLimit {
		return nil, supplementCapacityError(fmt.Sprintf("semantic evidence: %d bytes exceeds %d-byte safety limit", len(data), sourceLimit))
	}
	paths = nil
	for path := range neighbors {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := read(path); err != nil {
			if errors.Is(err, agent.ErrSupplementUnresolved) {
				continue
			}
			return nil, err
		}
		next, err := encode()
		if err != nil {
			return nil, err
		}
		if len(next) > sourceLimit {
			delete(sources, path)
			delete(hashes, path)
			delete(ranges, path)
		} else {
			data = next
		}
	}
	paths = nil
	for path := range owned {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	id := fmt.Sprintf("semantic-%x", sha256.Sum256([]byte(strings.Join(paths, "\n"))))[:25]
	verifiedSymbols := make(map[string][]agent.SupplementSymbol)
	for _, node := range index.Nodes {
		if _, included := sources[node.Path]; !included || node.DeclarationOnly || node.Line < 1 {
			continue
		}
		if node.Kind != "function" && node.Kind != "method" && node.Kind != "route" {
			continue
		}
		for _, span := range ranges[node.Path] {
			if span.Start <= node.Line && node.Line <= span.End {
				verifiedSymbols[node.Path] = append(verifiedSymbols[node.Path], agent.SupplementSymbol{
					Name: node.Name, Line: node.Line, Signature: node.Signature,
				})
				break
			}
		}
	}
	return &agent.LearningSupplement{Evidence: string(data), SourceHashes: hashes, SourceRanges: ranges, Symbols: verifiedSymbols, Focus: domain.EvidenceFocus{
		ID: id, Name: strings.Join(paths, ", "), EntryPaths: paths, Purpose: domain.EvidenceFocusPurposeCoverage,
		ScopeReason:   "Verify important business decisions and existing domain capabilities beyond static structure; omit generic implementation details.",
		AnalysisDepth: domain.EvidenceFocusDepthCareful,
	}}, nil
}

func supplementSource(path string, data []byte, ids []string, anchors []int, full bool, limit int, nodes map[string]codegraph.Node) (string, []agent.SourceRange, error) {
	if full && len(anchors) == 0 && len(data) <= limit || !full && len(ids) == 0 && len(anchors) == 0 {
		return string(data), []agent.SourceRange{{Start: 1, End: strings.Count(string(data), "\n") + 1}}, nil
	}
	if full && len(anchors) == 0 {
		return "", nil, supplementCapacityError(fmt.Sprintf("source evidence %s has no anchors and exceeds %d-byte safety limit", path, limit))
	}
	lines := strings.SplitAfter(string(data), "\n")
	pathNodes := make([]codegraph.Node, 0)
	if len(anchors) > 0 {
		for _, node := range nodes {
			if node.Path == path && node.Line > 0 && node.EndLine <= len(lines) {
				pathNodes = append(pathNodes, node)
			}
		}
	}
	sort.Slice(pathNodes, func(i, j int) bool {
		left, right := pathNodes[i].EndLine-pathNodes[i].Line, pathNodes[j].EndLine-pathNodes[j].Line
		if left != right {
			return left < right
		}
		return pathNodes[i].Line < pathNodes[j].Line
	})
	type span struct{ start, end int }
	var spans []span
	for _, id := range ids {
		node, ok := nodes[id]
		if !ok || node.Path != path || node.Line < 1 || node.EndLine < node.Line || node.EndLine > len(lines) {
			return "", nil, agent.ErrSupplementUnresolved
		}
		spans = append(spans, span{node.Line, node.EndLine})
	}
	for _, line := range anchors {
		if line < 1 || line > len(lines) {
			return "", nil, agent.ErrSupplementUnresolved
		}
		start, end := max(1, line-8), min(len(lines), line+8)
		for _, node := range pathNodes {
			if node.Line <= line && line <= node.EndLine {
				start, end = node.Line, node.EndLine
				break
			}
		}
		spans = append(spans, span{start, end})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var out strings.Builder
	var ranges []agent.SourceRange
	last := 0
	for _, current := range spans {
		if current.end <= last {
			continue
		}
		current.start = max(current.start, last+1)
		ranges = append(ranges, agent.SourceRange{Start: current.start, End: current.end})
		fmt.Fprintf(&out, "lines %d-%d:\n", current.start, current.end)
		for _, line := range lines[current.start-1 : current.end] {
			out.WriteString(line)
		}
		out.WriteByte('\n')
		last = current.end
	}
	return out.String(), ranges, nil
}

// supplementBatch 按完整证据大小动态组批；依赖关系只影响证据邻居排序，不强制拆分无直接关系的问题。
func supplementBatch(root string, index *codegraph.Index, gaps []semanticGap, patterns []domain.Pattern) (*agent.LearningSupplement, error) {
	selected := []semanticGap{gaps[0]}
	input, err := supplementEvidence(root, index, selected, patterns)
	if err != nil {
		return nil, err
	}
	for _, gap := range gaps[1:] {
		next, err := supplementEvidence(root, index, append(append([]semanticGap(nil), selected...), gap), patterns)
		if errors.Is(err, agent.ErrSupplementUnresolved) {
			continue
		}
		if err != nil {
			return nil, err
		}
		selected = append(selected, gap)
		input = next
	}
	return input, nil
}

func supplementLocalProject(ctx context.Context, cont *container.Container, opts learnCurrentProjectOptions, root string, index *codegraph.Index, result *learnCurrentProjectResult) error {
	return supplementLocalProjectWithSteps(ctx, cont, opts, root, index, result, nil)
}

func runLocalStage(steps *commandutil.ConsoleStepRunner, label string, fn func() error) error {
	if steps == nil {
		return fn()
	}
	return steps.Run(label, fn)
}

func supplementLocalProjectWithSteps(ctx context.Context, cont *container.Container, opts learnCurrentProjectOptions, root string, index *codegraph.Index, result *learnCurrentProjectResult, steps *commandutil.ConsoleStepRunner) error {
	if cont.FileTracker == nil || cont.PatternReader == nil {
		return nil
	}
	ctx = agent.WithCallBudget(ctx, cont.ConfigRepo.GetAgentConfig().Parallelism)
	baseCtx := ctx
	// 正常学习不设置总时长；单次 Agent 调用仍由 Agent 配置的 timeout 和重试策略保护。
	var changes *fileanalysis.FileChanges
	var patterns []domain.Pattern
	if err := runLocalStage(steps, i18n.Get("ProgressLearnCurrentDetectChanges"), func() error {
		var err error
		changes, err = fileanalysis.PrepareCurrentChangesWithOptions(baseCtx, cont.FileTracker, cont.ConfigRepo, root, root, domain.FileAnalysisScope{}, nil, fileanalysis.CurrentChangeOptions{Force: opts.force})
		if err != nil {
			return err
		}
		patterns, err = cont.PatternReader.GetAll(baseCtx)
		return err
	}); err != nil {
		return err
	}
	var status supplementStatus
	statusPath := filepath.Join(cont.SeedPath, "runtime", "local-supplement.json")
	var previous supplementStatus
	if data, err := os.ReadFile(statusPath); err == nil {
		_ = json.Unmarshal(data, &previous)
	}
	screenReasons := make(map[string]string)
	save := func() error {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(statusPath), 0700); err != nil {
			return err
		}
		return fileio.WriteFileAtomic(statusPath, data, 0600)
	}
	if err := runLocalStage(steps, i18n.Get("LearnLocalPlan"), func() error {
		changed := make(map[string]bool)
		hashes := make(map[string]string, len(changes.Records))
		for _, path := range changes.AddedOrModified {
			changed[path] = true
		}
		for _, record := range changes.Records {
			hashes[record.Path] = record.Hash
		}
		status = supplementStatus{
			IndexedFiles: len(index.Files), UnindexedFiles: len(index.Unindexed),
			ChangedFiles: len(changes.AddedOrModified), Pending: selectSemanticGaps(index, changed, patterns, changes.PreviousAnalyzedCount == 0),
			Blocked: make(map[string]supplementBlock),
		}
		if !opts.force && !opts.semantic && len(opts.focusPaths) == 0 {
			for _, gap := range status.Pending {
				block := previous.Blocked[gap.Path]
				if hashes[gap.Path] != "" && block.SourceHash == hashes[gap.Path] && block.InputHash == supplementInputHash(index, gap, patterns) && block.Revision == supplementEvidenceRevision {
					status.Blocked[gap.Path] = block
					status.LastError = block.Reason
				}
			}
		}
		result.changedCount = len(changes.AddedOrModified)
		result.deletedCount = len(changes.Deleted)
		if len(opts.focusPaths) > 0 {
			focuses, err := resolveFocusPaths(root, opts.focusPaths)
			if err != nil {
				return err
			}
			status.Pending = nil
			files := append([]codegraph.File(nil), index.Files...)
			for _, path := range index.Unindexed {
				files = append(files, codegraph.File{Path: path})
			}
			for _, file := range files {
				for _, focus := range focuses {
					rel, err := filepath.Rel(focus, filepath.Join(root, filepath.FromSlash(file.Path)))
					if err == nil && (rel == "." || filepath.IsLocal(rel)) && (changed[file.Path] || opts.force || opts.semantic) {
						status.Pending = append(status.Pending, semanticGap{Path: file.Path, Reason: "explicit request: confirm important business semantics using current evidence"})
						break
					}
				}
			}
		}
		status.SemanticCandidates = len(status.Pending)
		if err := save(); err != nil {
			return err
		}
		if len(opts.focusPaths) > 0 || opts.force {
			return nil
		}
		screener, ok := cont.Agent.(agent.SemanticScreener)
		if !ok || !cont.Agent.IsAvailable() {
			return nil
		}
		var unscreened []semanticGap
		status.Screened = make(map[string]string)
		for _, gap := range status.Pending {
			if gap.Existing || status.Blocked[gap.Path].SourceHash != "" {
				continue
			}
			if hash := hashes[gap.Path]; hash != "" && previous.Screened[gap.Path] == hash {
				status.Screened[gap.Path] = hash
				continue
			}
			unscreened = append(unscreened, gap)
		}
		if len(unscreened) == 0 {
			return save()
		}
		if batch, err := supplementBatch(root, index, unscreened, patterns); err == nil && len(batch.Focus.EntryPaths) == len(unscreened) {
			return nil
		}
		skipped, err := screenCandidates(ctx, screener, index, unscreened, cont.ConfigRepo.GetAgentConfig().Parallelism)
		if err != nil {
			logger.Warn(err.Error())
			return nil
		}
		remaining := status.Pending[:0]
		for _, gap := range status.Pending {
			if reason, skippedHere := skipped[gap.Path]; skippedHere && !gap.Existing {
				screenReasons[gap.Path] = reason
				status.ScreenedOut++
				continue
			}
			remaining = append(remaining, gap)
			if !gap.Existing && hashes[gap.Path] != "" {
				status.Screened[gap.Path] = hashes[gap.Path]
			}
		}
		status.Pending = remaining
		status.SemanticCandidates = len(remaining)
		if err := save(); err != nil {
			return err
		}
		logger.Info(i18n.GetWithParams("LearnLocalScreenComplete", map[string]interface{}{
			"Reviewed": len(unscreened), "Skipped": len(skipped), "Retained": len(remaining),
		}))
		return nil
	}); err != nil {
		return err
	}
	// 结构覆盖与语义确认分别记录；本地已判定无需 AI 的文件不在下次 sync 重复入选。
	if err := runLocalStage(steps, i18n.Get("LearnLocalCommitCoverage"), func() error {
		semanticPaths := make(map[string]bool, len(status.Pending))
		for _, gap := range status.Pending {
			semanticPaths[gap.Path] = true
		}
		localChanges := &fileanalysis.FileChanges{Scope: changes.Scope, Deleted: changes.Deleted}
		for _, record := range changes.Records {
			if semanticPaths[record.Path] {
				continue
			}
			record.AnalysisStatus = domain.FileAnalysisStatusSelectionSkipped
			record.SelectionReason = "local scan found no automatic semantic gap"
			if reason := screenReasons[record.Path]; reason != "" {
				record.SelectionReason = "semantic screen: " + reason
			}
			localChanges.Records = append(localChanges.Records, record)
		}
		return fileanalysis.CommitCurrentChanges(ctx, cont.FileTracker, localChanges)
	}); err != nil {
		return err
	}
	pending := make([]semanticGap, 0, len(status.Pending))
	for _, gap := range status.Pending {
		if status.Blocked[gap.Path].SourceHash == "" {
			pending = append(pending, gap)
		}
	}
	supplementLabel := i18n.Get("LearnLocalSupplement")
	processed := len(status.Pending) - len(pending)
	if err := runLocalStage(steps, supplementLabel, func() error {
		var jobs []supplementJob
		for len(pending) > 0 {
			if err := ctx.Err(); err != nil {
				status.LastError = err.Error()
				break
			}
			input, err := supplementBatch(root, index, pending, patterns)
			if errors.Is(err, agent.ErrSupplementUnresolved) {
				status.LastError = err.Error()
				gap := pending[0]
				if errors.Is(err, errSupplementEvidenceCapacity) {
					for _, record := range changes.Records {
						if record.Path == gap.Path && record.Hash != "" {
							status.Blocked[gap.Path] = supplementBlock{SourceHash: record.Hash, InputHash: supplementInputHash(index, gap, patterns), Reason: err.Error(), Revision: supplementEvidenceRevision}
							break
						}
					}
				}
				pending = pending[1:]
				processed++
				continue
			}
			if err != nil {
				return err
			}
			owned := make(map[string]bool)
			for _, path := range input.Focus.EntryPaths {
				owned[path] = true
			}
			remaining := pending[:0]
			for _, gap := range pending {
				if !owned[gap.Path] {
					remaining = append(remaining, gap)
				}
			}
			pending = remaining
			jobs = append(jobs, supplementJob{number: len(jobs) + 1, input: input})
		}
		if len(jobs) == 0 {
			return nil
		}
		if err := commandutil.RequireAgentAvailable(cont); err != nil {
			status.LastError = err.Error()
			return nil
		}
		if steps != nil {
			steps.Detail(supplementLabel, i18n.GetWithParams("LearnLocalSupplementDetail", map[string]interface{}{
				"Batch": 1, "Completed": processed, "Total": status.SemanticCandidates,
			}))
		}
		outcomes := runSupplementJobs(ctx, cont, opts, jobs)
		var saveErr error
		for range jobs {
			outcome := <-outcomes
			if outcome.started {
				processed += len(outcome.job.input.Focus.EntryPaths)
			}
			if steps != nil && outcome.started {
				steps.Detail(supplementLabel, i18n.GetWithParams("LearnLocalSupplementDetail", map[string]interface{}{
					"Batch": outcome.job.number, "Completed": processed, "Total": status.SemanticCandidates,
				}))
			}
			if outcome.err != nil {
				status.LastError = outcome.err.Error()
				continue
			}
			learned := outcome.result
			result.patternsCount += learned.patternsCount
			result.savedCount += learned.savedCount
			result.retiredCount += learned.retiredCount
			result.droppedCount += learned.droppedCount
			result.dropped = append(result.dropped, learned.dropped...)
			result.skipped = result.skipped && learned.skipped
			owned := make(map[string]bool, len(outcome.job.input.Focus.EntryPaths))
			for _, path := range outcome.job.input.Focus.EntryPaths {
				owned[path] = true
			}
			remaining := status.Pending[:0]
			for _, gap := range status.Pending {
				if !owned[gap.Path] {
					remaining = append(remaining, gap)
				}
			}
			status.Pending = remaining
			if len(status.Pending) == 0 {
				status.LastError = ""
			}
			if err := save(); err != nil && saveErr == nil {
				saveErr = err
			}
		}
		return saveErr
	}); err != nil {
		return err
	}
	return runLocalStage(steps, i18n.Get("LearnLocalFinalize"), func() error {
		if err := save(); err != nil {
			return err
		}
		if errors.Is(baseCtx.Err(), context.Canceled) {
			return baseCtx.Err()
		}
		result.pendingCount = len(status.Pending)
		if len(status.Pending) > 0 {
			logger.Warn(i18n.GetWithParams("LearnLocalPending", map[string]interface{}{"Count": len(status.Pending), "Path": statusPath, "Reason": status.LastError}))
		}
		return nil
	})
}
