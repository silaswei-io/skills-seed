package learncurrent

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/runjournal"
	"github.com/silaswei-io/skills-seed/internal/runtimecontext"
	"github.com/silaswei-io/skills-seed/internal/service/patternnorm"
)

// 顶层阶段名：稳定、可跨版本对比，不绑定 i18n 文案。
const (
	stagePrepare   = "prepare"
	stageDetect    = "detect"
	stagePlan      = "plan"
	stageAnalyze   = "analyze"
	stageNormalize = "normalize"
	stageProfile   = "profile"
)

// learnRunObserver 是单次 learn current 的运行观察者。
// 只收集事实，不参与编排决策；早停由 early_stop 策略决定后再通知观察者。
type learnRunObserver struct {
	mu            sync.Mutex
	stageStarted  map[string]time.Time
	stageMs       map[string]int64
	skippedStages map[string]struct{}
	analysisMode  string
	agentCounter  *runtimecontext.AgentCallCounter
}

func newLearnRunObserver(counter *runtimecontext.AgentCallCounter) *learnRunObserver {
	return &learnRunObserver{
		stageStarted:  make(map[string]time.Time),
		stageMs:       make(map[string]int64),
		skippedStages: make(map[string]struct{}),
		agentCounter:  counter,
	}
}

func (o *learnRunObserver) startStage(name string) {
	if o == nil || strings.TrimSpace(name) == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.stageStarted[name] = time.Now()
}

func (o *learnRunObserver) endStage(name string) {
	if o == nil || strings.TrimSpace(name) == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	started, ok := o.stageStarted[name]
	if !ok {
		return
	}
	delete(o.stageStarted, name)
	o.stageMs[name] += time.Since(started).Milliseconds()
}

func (o *learnRunObserver) noteSkip(name string) {
	if o == nil || strings.TrimSpace(name) == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.skippedStages[name] = struct{}{}
}

func (o *learnRunObserver) noteAnalysisMode(mode string) {
	if o == nil {
		return
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	switch {
	case o.analysisMode == "":
		o.analysisMode = mode
	case o.analysisMode != mode:
		o.analysisMode = "mixed"
	}
}

func (o *learnRunObserver) skippedList() []string {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.skippedStages) == 0 {
		return nil
	}
	out := make([]string, 0, len(o.skippedStages))
	for name := range o.skippedStages {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (o *learnRunObserver) analysisModeValue() string {
	if o == nil {
		return ""
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.analysisMode
}

func (o *learnRunObserver) agentCallTotal() int {
	if o == nil || o.agentCounter == nil {
		return 0
	}
	return o.agentCounter.Total()
}

// buildJournalMetrics 把观察者状态与运行结果投影为可持久化度量。
func (o *learnRunObserver) buildJournalMetrics(result *learnCurrentProjectResult, wall time.Duration) *runjournal.Metrics {
	if o == nil && result == nil {
		return nil
	}
	out := &runjournal.Metrics{WallMs: wall.Milliseconds()}
	if result != nil {
		out.Focuses = result.focusCount
		out.CandidatesFound = result.patternsCount
		out.Saved = result.savedCount
		out.Retired = result.retiredCount
		out.Dropped = result.droppedCount
		out.DropReasonCodes = dropReasonCodeCounts(result.dropped)
		out.ChangeProfile = result.changeProfile
		out.LearningMode = result.learningMode
		out.Resumed = result.resumed
		out.AnalysisMode = result.analysisMode
		out.SkippedStages = append([]string(nil), result.skippedStages...)
	}
	if o != nil {
		o.mu.Lock()
		if len(o.stageMs) > 0 {
			out.StageMs = make(map[string]int64, len(o.stageMs))
			for k, v := range o.stageMs {
				out.StageMs[k] = v
			}
		}
		if out.AnalysisMode == "" {
			out.AnalysisMode = o.analysisMode
		}
		if len(o.skippedStages) > 0 {
			extra := make([]string, 0, len(o.skippedStages))
			for name := range o.skippedStages {
				extra = append(extra, name)
			}
			out.SkippedStages = uniqueSortedStrings(append(out.SkippedStages, extra...))
		}
		counter := o.agentCounter
		o.mu.Unlock()
		if counter != nil {
			out.AgentCalls = counter.Snapshot()
			out.AgentCallTotal = counter.Total()
		}
	}
	if metricsEmpty(out) {
		return nil
	}
	return out
}

func metricsEmpty(m *runjournal.Metrics) bool {
	if m == nil {
		return true
	}
	return m.WallMs == 0 &&
		len(m.StageMs) == 0 &&
		m.AgentCallTotal == 0 &&
		m.Focuses == 0 &&
		m.CandidatesFound == 0 &&
		m.Saved == 0 &&
		m.Retired == 0 &&
		m.Dropped == 0 &&
		!m.Resumed &&
		m.AnalysisMode == "" &&
		len(m.SkippedStages) == 0
}

func dropReasonCodeCounts(dropped []patternnorm.Drop) map[string]int {
	if len(dropped) == 0 {
		return nil
	}
	out := make(map[string]int, len(dropped))
	for _, item := range dropped {
		code := strings.TrimSpace(string(item.ReasonCode))
		if code == "" {
			code = "unspecified"
		}
		out[code]++
	}
	return out
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
