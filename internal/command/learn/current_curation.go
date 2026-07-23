package learn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
)

type currentCurationCheckpoint struct {
	repo     *commandstate.Repository
	state    *commandstate.State
	imported *agent.CuratePatternsResult
}

func newCurrentCurationCheckpoint(repo *commandstate.Repository, state *commandstate.State, imported *agent.CuratePatternsResult) *currentCurationCheckpoint {
	return &currentCurationCheckpoint{repo: repo, state: state, imported: imported}
}

func (r *learnCurrentProjectRun) loadImportedCuration() error {
	path := strings.TrimSpace(r.opts.curationOutput)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read curation output: %w", err)
	}
	result, err := parser.ParseCuratePatternsArtifact(string(data))
	if err != nil {
		return fmt.Errorf("parse curation output: %w", err)
	}
	if err := agent.RequireResult(result, "CuratePatterns"); err != nil {
		return fmt.Errorf("parse curation output: %w", err)
	}
	r.importedCuration = result
	return nil
}

func (c *currentCurationCheckpoint) Load(ctx context.Context, candidateHash string) (*agent.CuratePatternsResult, bool, error) {
	if c == nil || c.state == nil {
		return nil, false, nil
	}
	if c.imported != nil {
		if err := c.Save(ctx, candidateHash, c.imported); err != nil {
			return nil, false, err
		}
		return c.imported, true, nil
	}
	if checkpoint := c.state.Curation; checkpoint != nil {
		if checkpoint.CandidateHash != candidateHash {
			return nil, false, fmt.Errorf("curation candidate hash changed")
		}
		var result agent.CuratePatternsResult
		if err := json.Unmarshal(checkpoint.Decision, &result); err != nil {
			return nil, false, fmt.Errorf("decode curation decision: %w", err)
		}
		return &result, true, nil
	}
	return nil, false, nil
}

func (c *currentCurationCheckpoint) Save(ctx context.Context, candidateHash string, result *agent.CuratePatternsResult) error {
	if c == nil || c.state == nil || c.repo == nil {
		return fmt.Errorf("curation checkpoint is not initialized")
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode curation decision: %w", err)
	}
	c.state.Curation = &commandstate.CurationCheckpoint{
		CandidateHash: candidateHash,
		Decision:      data,
	}
	return c.repo.Save(ctx, c.state)
}
