package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/codegraph"
)

// structuralContextRequest 只选择已有索引中的证据，不启动第二套解析器。
type structuralContextRequest struct {
	SeedPaths []string
}

func (s *AnalyzerService) collectStructuralContext(ctx context.Context, root string, req structuralContextRequest) (string, error) {
	if input := agent.CurrentLearningSupplement(ctx); input != nil {
		return input.Evidence, nil
	}
	index, err := codegraph.Load(root)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var nodes []codegraph.Node
	for _, node := range index.Nodes {
		if pathInFocus(node.Path, req.SeedPaths) {
			nodes = append(nodes, node)
		}
	}
	data, err := json.Marshal(nodes)
	return string(data), err
}
