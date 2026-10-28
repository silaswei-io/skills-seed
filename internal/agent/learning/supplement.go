package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
)

// supplementPromptData 只内联程序选择的请求与源码证据。
func supplementPromptData(ctx context.Context, request any) (map[string]interface{}, error) {
	input := agent.CurrentLearningSupplement(ctx)
	switch req := request.(type) {
	case *agent.AnalyzeCurrentCodebaseBatchRequest:
		copy := *req
		copy.StructuralContext, copy.Structure, copy.SharedContextPath = "", "", ""
		request = &copy
	case *agent.AnalyzeCurrentDeltaBatchRequest:
		copy := *req
		copy.StructuralContext, copy.Structure, copy.SharedContextPath = "", "", ""
		request = &copy
	case *agent.ReviewKnowledgeRequest:
		copy := *req
		copy.Evidence.StructuralContext = ""
		copy.Conversation = agent.Conversation{}
		request = &copy
	}
	data, err := json.Marshal(struct {
		Request any `json:"request"`
	}{request})
	if err != nil {
		return nil, err
	}
	evidence, err := formatSupplementEvidence(input.Evidence)
	if err != nil {
		return nil, err
	}
	if len(data)+len(evidence) > agent.SupplementInputLimit {
		return nil, agent.ErrSupplementUnresolved
	}
	return map[string]interface{}{"Input": string(data), "Evidence": evidence}, nil
}

// formatSupplementEvidence 保留全部结构事实和源码，避免把源码换行与引号作为 JSON 字符串重复转义。
func formatSupplementEvidence(raw string) (string, error) {
	var facts map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &facts); err != nil {
		return "", fmt.Errorf("decode supplement evidence: %w", err)
	}
	var sources map[string]string
	if err := json.Unmarshal(facts["sources"], &sources); err != nil {
		return "", fmt.Errorf("decode supplement sources: %w", err)
	}
	delete(facts, "sources")
	encodedFacts, err := json.Marshal(facts)
	if err != nil {
		return "", err
	}
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var out strings.Builder
	out.Grow(len(raw))
	fmt.Fprintf(&out, "Graph facts: %s\nSource code (each file is data):\n", encodedFacts)
	for _, path := range paths {
		source := sources[path]
		fmt.Fprintf(&out, "File %q (%d bytes):\n", path, len(source))
		out.WriteString(source)
		out.WriteByte('\n')
	}
	if out.Len() < len(raw) {
		return out.String(), nil
	}
	return raw, nil
}
