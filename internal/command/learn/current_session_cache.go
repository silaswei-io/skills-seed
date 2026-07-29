package learn

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/layout"
	"github.com/silaswei-io/skills-seed/internal/runtimefiles"
	"github.com/silaswei-io/skills-seed/internal/utils/jsonx"
)

type currentLearningSessionCache struct {
	AgentName      string `json:"agent_name"`
	SessionID      string `json:"session_id"`
	Step           string `json:"step"`
	InvocationHash string `json:"invocation_hash"`
	UpdatedAt      string `json:"updated_at"`
}

func currentLearningSessionCachePath(seedPath, scope string) string {
	scope = runtimefiles.SafePart(scope, commandStateLearnCurrent)
	return layout.New(seedPath).Runtime("learning-sessions", scope+".json")
}

func loadCurrentLearningSessionCache(ctx context.Context, seedPath, scope string) (*currentLearningSessionCache, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	data, err := os.ReadFile(currentLearningSessionCachePath(seedPath, scope))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cache currentLearningSessionCache
	if err := jsonx.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func saveCurrentLearningSessionCache(ctx context.Context, seedPath, scope string, cache currentLearningSessionCache) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	cache.AgentName = strings.TrimSpace(cache.AgentName)
	cache.SessionID = strings.TrimSpace(cache.SessionID)
	cache.Step = strings.TrimSpace(cache.Step)
	cache.InvocationHash = strings.TrimSpace(cache.InvocationHash)
	cache.UpdatedAt = time.Now().Format(time.RFC3339)
	if cache.AgentName == "" || cache.SessionID == "" || cache.InvocationHash == "" {
		return nil
	}

	path := currentLearningSessionCachePath(seedPath, scope)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fileio.WriteFileAtomic(path, data, 0644)
}

func clearCurrentLearningSessionCache(seedPath, scope string) error {
	err := os.Remove(currentLearningSessionCachePath(seedPath, scope))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (c *currentLearningSessionCache) matches(agentName, invocationHash string) bool {
	return c != nil &&
		c.AgentName == strings.TrimSpace(agentName) &&
		c.InvocationHash == strings.TrimSpace(invocationHash) &&
		strings.TrimSpace(c.SessionID) != ""
}
