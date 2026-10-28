package learncurrent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/codegraph"
)

// screenCandidates 仅用紧凑图事实排除明确没有长期价值的项；不确定项保留源码分析。
func screenCandidates(ctx context.Context, screener agent.SemanticScreener, index *codegraph.Index, gaps []semanticGap, parallelism int) (map[string]string, error) {
	nodes := make(map[string]codegraph.Node, len(index.Nodes))
	for _, node := range index.Nodes {
		nodes[node.ID] = node
	}
	callers, callees := make(map[string][]string), make(map[string][]string)
	for _, call := range index.Calls {
		source, sourceOK := nodes[call.Source]
		target, targetOK := nodes[call.Target]
		if sourceOK && targetOK {
			callers[target.ID] = append(callers[target.ID], source.Path+"::"+source.Name)
			callees[source.ID] = append(callees[source.ID], target.Path+"::"+target.Name)
		}
	}
	var facts []agent.SemanticScreenCandidate
	for _, gap := range gaps {
		fact := agent.SemanticScreenCandidate{Path: gap.Path}
		protected := gap.Existing
		for _, id := range gap.Symbols {
			node, ok := nodes[id]
			if !ok {
				continue
			}
			protected = protected || node.Kind == "route"
			signature := node.Signature
			if signature == "" {
				signature = node.Name
			}
			fact.Symbols = append(fact.Symbols, node.Kind+" "+signature)
			fact.Callers = append(fact.Callers, callers[id]...)
			fact.Callees = append(fact.Callees, callees[id]...)
		}
		if protected || len(fact.Symbols) == 0 {
			continue
		}
		sort.Strings(fact.Callers)
		sort.Strings(fact.Callees)
		fact.Callers = slices.Compact(fact.Callers)
		fact.Callees = slices.Compact(fact.Callees)
		facts = append(facts, fact)
	}
	var requests []*agent.SemanticScreenRequest
	for len(facts) > 0 {
		request := &agent.SemanticScreenRequest{}
		for _, fact := range facts {
			request.Candidates = append(request.Candidates, fact)
			data, err := json.Marshal(request)
			if err != nil {
				return nil, err
			}
			if len(data) > agent.SupplementInputLimit/2 {
				request.Candidates = request.Candidates[:len(request.Candidates)-1]
				break
			}
		}
		if len(request.Candidates) == 0 {
			facts = facts[1:]
			continue
		}
		facts = facts[len(request.Candidates):]
		requests = append(requests, request)
	}
	if len(requests) == 0 {
		return map[string]string{}, nil
	}
	type outcome struct {
		request *agent.SemanticScreenRequest
		result  *agent.SemanticScreenResult
		err     error
	}
	queue := make(chan *agent.SemanticScreenRequest, len(requests))
	results := make(chan outcome, len(requests))
	for _, request := range requests {
		queue <- request
	}
	close(queue)
	for range min(agent.EffectiveParallelism(parallelism), len(requests)) {
		go func() {
			for request := range queue {
				if err := ctx.Err(); err != nil {
					results <- outcome{request: request, err: err}
					continue
				}
				result, err := screener.ScreenSemanticGaps(ctx, request)
				results <- outcome{request: request, result: result, err: err}
			}
		}()
	}
	skip := make(map[string]string)
	var firstErr error
	for range requests {
		item := <-results
		if item.err != nil {
			firstErr = errors.Join(firstErr, item.err)
			continue
		}
		if item.result == nil {
			firstErr = errors.Join(firstErr, errors.New("semantic screening returned no result"))
			continue
		}
		allowed := make(map[string]bool, len(item.request.Candidates))
		for _, fact := range item.request.Candidates {
			allowed[fact.Path] = true
		}
		for _, decision := range item.result.Skip {
			if !allowed[decision.Path] || strings.TrimSpace(decision.Reason) == "" || skip[decision.Path] != "" {
				firstErr = errors.Join(firstErr, fmt.Errorf("semantic screening returned invalid path or reason: %q", decision.Path))
				continue
			}
			skip[decision.Path] = decision.Reason
		}
	}
	if firstErr != nil {
		return nil, fmt.Errorf("semantic screening: %w", firstErr)
	}
	return skip, nil
}
