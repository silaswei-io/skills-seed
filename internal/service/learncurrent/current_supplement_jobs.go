package learncurrent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
)

type supplementJob struct {
	number int
	input  *agent.LearningSupplement
}

type supplementOutcome struct {
	job     supplementJob
	result  *learnCurrentProjectResult
	err     error
	started bool
}

// runSupplementJobs 只并行独立证据批次；调用上限与工作区共用 Agent budget。
func runSupplementJobs(ctx context.Context, cont *container.Container, opts learnCurrentProjectOptions, jobs []supplementJob) <-chan supplementOutcome {
	outcomes := make(chan supplementOutcome, len(jobs))
	parallelism := agent.EffectiveParallelism(cont.ConfigRepo.GetAgentConfig().Parallelism)
	queue := make(chan supplementJob, len(jobs))
	for _, job := range jobs {
		queue <- job
	}
	close(queue)
	for range min(parallelism, len(jobs)) {
		go func() {
			for job := range queue {
				if err := ctx.Err(); err != nil {
					outcomes <- supplementOutcome{job: job, err: err}
					continue
				}
				result, err := runSupplementJob(ctx, cont, opts, job.input)
				outcomes <- supplementOutcome{job: job, result: result, err: err, started: true}
			}
		}()
	}
	return outcomes
}

func runSupplementJob(ctx context.Context, cont *container.Container, opts learnCurrentProjectOptions, input *agent.LearningSupplement) (*learnCurrentProjectResult, error) {
	if err := commandutil.RequireAgentAvailable(cont); err != nil {
		return nil, err
	}
	semanticOpts := opts
	semanticOpts.semantic = true
	semanticOpts.force = opts.force || opts.semantic
	semanticOpts.skipRuntimeCleanup = true
	semanticOpts.userContext = fmt.Sprintf("%s\nLocal evidence revision: %x", opts.userContext, sha256.Sum256([]byte(input.Evidence)))
	semanticOpts.focusPaths = input.Focus.EntryPaths
	semanticOpts.stateScope = strings.Trim(opts.stateScope, "-") + "-" + input.Focus.ID
	semanticOpts.showProgress = false
	semanticOpts.showDetailedLogs = false
	semanticOpts.onStepStart = nil
	semanticOpts.onStepUpdate = nil
	semanticOpts.onStepComplete = nil
	return runSemanticSupplement(agent.WithLearningSupplement(ctx, input), cont, semanticOpts)
}
