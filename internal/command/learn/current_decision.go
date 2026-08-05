package learn

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/commandstate"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
)

type currentDecisionCheckpoint struct {
	repo  *commandstate.Repository
	state *commandstate.State
}

func newCurrentDecisionCheckpoint(repo *commandstate.Repository, state *commandstate.State) *currentDecisionCheckpoint {
	return &currentDecisionCheckpoint{repo: repo, state: state}
}

func (c *currentDecisionCheckpoint) Load(ctx context.Context, candidateHash string) (*patternnorm.Decision, bool, error) {
	if c == nil || c.state == nil {
		return nil, false, nil
	}
	if checkpoint := c.state.Decision; checkpoint != nil {
		if checkpoint.CandidateHash != candidateHash {
			return nil, false, fmt.Errorf("%s", i18n.Get("LearnCurrentDecisionCandidateChanged"))
		}
		var result patternnorm.Decision
		if err := json.Unmarshal(checkpoint.Decision, &result); err != nil {
			return nil, false, fmt.Errorf("%s: %w", i18n.Get("LearnCurrentDecodeDecisionFailed"), err)
		}
		return &result, true, nil
	}
	return nil, false, nil
}

func (c *currentDecisionCheckpoint) Save(ctx context.Context, candidateHash string, result *patternnorm.Decision) error {
	if c == nil || c.state == nil || c.repo == nil {
		return fmt.Errorf("%s", i18n.Get("LearnCurrentDecisionCheckpointMissing"))
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Get("LearnCurrentEncodeDecisionFailed"), err)
	}
	c.state.Decision = &commandstate.DecisionCheckpoint{
		CandidateHash: candidateHash,
		Decision:      data,
	}
	return c.repo.Save(ctx, c.state)
}
