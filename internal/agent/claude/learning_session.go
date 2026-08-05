package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/agent/aicontract"
	"github.com/silaswei-io/skills-seed/internal/agent/parser"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	promptloader "github.com/silaswei-io/skills-seed/internal/prompts"
)

type learningSession struct {
	agent     *ClaudeAgent
	sessionID string
	resume    bool
	mu        sync.Mutex
}

func (c *ClaudeAgent) StartLearningSession(ctx context.Context, req agent.LearningSessionRequest) (agent.LearningSession, error) {
	if sessionID := strings.TrimSpace(req.ResumeSessionID); sessionID != "" {
		return &learningSession{agent: c, sessionID: sessionID, resume: true}, nil
	}
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-conversation-start", req.Stage))
	inputs, err := agent.NewPromptInputSessionForContext(ctx, agent.RuntimePromptInputPrefix("skills-seed-learning-conversation-start", req.Stage))
	if err != nil {
		return nil, err
	}
	defer inputs.Cleanup()

	data, err := agent.LearningSessionPromptData(inputs, req)
	if err != nil {
		return nil, err
	}
	prompt, err := c.promptLoader.RenderForRuntimeTask("learning-conversation-start", data, promptRuntimeTask(task))
	if err != nil || prompt == "" {
		return nil, errors.New(i18n.Get("AgentRenderInitSkillsPromptFailed"))
	}

	output, sessionID, _, err := c.callClaudeNewSession(ctx, agent.OperationLearningConversationStart, prompt, aicontract.ContractLearningSessionAck, task)
	if err != nil {
		return nil, err
	}
	if err := parser.ParseLearningSessionAck(output); err != nil {
		return nil, err
	}
	return &learningSession{agent: c, sessionID: sessionID, resume: true}, nil
}

func (s *learningSession) SessionID() string {
	return s.sessionID
}

func (s *learningSession) SelectLearningCandidates(ctx context.Context, req *agent.SelectLearningCandidatesRequest) (*agent.SelectLearningCandidatesResult, error) {
	output, err := s.call(ctx, "LearningCandidateSelect", "learning-candidate-select", "skills-seed-learning-candidate-select", aicontract.ContractSelectLearningCandidates, agent.NewRuntimeTask(agent.RuntimeSlug("learning-candidate-select", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.SelectLearningCandidatesPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParseSelectLearningCandidatesResult(output)
}

func (s *learningSession) PlanLearningAgenda(ctx context.Context, req *agent.PlanLearningAgendaRequest) (*agent.PlanLearningAgendaResult, error) {
	output, err := s.call(ctx, "LearningPackPlan", "learning-pack-plan", "skills-seed-learning-pack-plan", aicontract.ContractPlanLearningAgenda, agent.NewRuntimeTask(agent.RuntimeSlug("learning-pack-plan", "")), func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.PlanLearningAgendaPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	return parser.ParsePlanLearningAgendaResult(output)
}

func (s *learningSession) AnalyzeCurrentCodebaseBatch(ctx context.Context, req *agent.AnalyzeCurrentCodebaseBatchRequest) (*agent.AnalyzeCurrentCodebaseBatchResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-pack-analyze", req.RuntimeLabel))
	output, err := s.call(ctx, agent.AnalyzeCurrentCodebaseBatchOperation(req), "learning-pack-analyze", agent.RuntimePromptInputPrefix("skills-seed-learning-pack-analyze", req.RuntimeLabel), aicontract.ContractAnalyzeCurrentCodebaseBatch, task, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.AnalyzeCurrentCodebaseBatchPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseAnalyzeCurrentCodebaseBatchResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "AnalyzeCurrentCodebaseBatch")
}

func (s *learningSession) AnalyzeCurrentDeltaBatch(ctx context.Context, req *agent.AnalyzeCurrentDeltaBatchRequest) (*agent.AnalyzeCurrentDeltaBatchResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-delta-pack-analyze", req.RuntimeLabel))
	output, err := s.call(ctx, agent.AnalyzeCurrentDeltaBatchOperation(req), "learning-delta-pack-analyze", agent.RuntimePromptInputPrefix("skills-seed-learning-delta-pack-analyze", req.RuntimeLabel), aicontract.ContractAnalyzeCurrentDeltaBatch, task, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.AnalyzeCurrentDeltaBatchPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseAnalyzeCurrentDeltaBatchResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "AnalyzeCurrentDeltaBatch")
}

func (s *learningSession) RefreshProjectProfile(ctx context.Context, req *agent.AnalyzeProjectRequest) (*agent.AnalyzeProjectResult, error) {
	task := agent.NewRuntimeTask(agent.RuntimeSlug("learning-profile-refresh", ""))
	output, err := s.call(ctx, "LearningProfileRefresh", "learning-profile-refresh", "skills-seed-learning-profile-refresh", aicontract.ContractProjectProfile, task, func(inputs *agent.PromptInputSession) (map[string]interface{}, error) {
		return agent.AnalyzeProjectPromptData(inputs, req)
	})
	if err != nil {
		return nil, err
	}
	result, err := parser.ParseAnalyzeProjectResult(output)
	if err != nil {
		return nil, err
	}
	return result, agent.RequireResult(result, "AnalyzeProject")
}

func (s *learningSession) Close(context.Context) error {
	return nil
}

func (s *learningSession) call(ctx context.Context, operation, templateName, inputPrefix, outputContract string, task agent.RuntimeTask, build func(*agent.PromptInputSession) (map[string]interface{}, error)) (string, error) {
	inputs, err := agent.NewPromptInputSessionForContext(ctx, inputPrefix)
	if err != nil {
		return "", err
	}
	defer inputs.Cleanup()

	data, err := build(inputs)
	if err != nil {
		return "", err
	}
	return s.callPrepared(ctx, operation, templateName, outputContract, task, data)
}

func (s *learningSession) callPrepared(ctx context.Context, operation, templateName, outputContract string, task agent.RuntimeTask, data interface{}) (string, error) {
	prompt, err := s.agent.promptLoader.RenderForRuntimeTask(templateName, data, promptRuntimeTask(task))
	if err != nil || prompt == "" {
		return "", errors.New(i18n.Get("AgentRenderInitSkillsPromptFailed"))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	output, _, err := s.agent.callClaudeSession(ctx, operation, prompt, outputContract, s.sessionID, s.resume, task)
	if err != nil {
		return "", err
	}
	s.resume = true
	return output, nil
}

func newClaudeSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("%s: %w", i18n.Get("AgentClaudeLearningSessionIDFailed"), err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexed[0:8], hexed[8:12], hexed[12:16], hexed[16:20], hexed[20:32]), nil
}

func promptRuntimeTask(task agent.RuntimeTask) promptloader.RuntimeTask {
	return promptloader.RuntimeTask{ID: task.ID, Slug: task.Slug}
}
