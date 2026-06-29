package progress

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCompleteStepKeepsFinalElapsedTime(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := New(1)
		tracker.enabled = true

		tracker.StartStep("load")

		tracker.mu.Lock()
		tracker.startedAt = time.Now().Add(-1500 * time.Millisecond)
		tracker.mu.Unlock()

		tracker.CompleteStep("load")
	})

	if !strings.Contains(output, "(1s)") {
		t.Fatalf("expected completed progress output to include final elapsed time, got %q", output)
	}
}

func TestRenderOmitsPercentWhenStepCountsAreDisplayed(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := New(4)
		tracker.done = 1
		tracker.active = true
		tracker.label = "analyze"
		tracker.frame = 3
		tracker.startedAt = time.Now()

		tracker.renderLocked(false)
	})

	if strings.Contains(output, "%") {
		t.Fatalf("expected progress output to omit percent when step count is shown, got %q", output)
	}
	if !strings.Contains(output, "[#######---------------------] 2/4 \\ analyze") {
		t.Fatalf("expected progress output to keep bar, step count, spinner, and label, got %q", output)
	}
}

func TestPrintConsoleLineDefersActiveProgressLineConsoleOutput(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := New(1)
		tracker.enabled = true

		tracker.StartStep("分析当前代码库")
		PrintConsoleLine("Token 消耗: 本次 575.5k")
		tracker.CompleteStep("分析当前代码库")
	})

	completedIndex := strings.Index(output, "[############################] 1/1   分析当前代码库")
	if completedIndex < 0 {
		t.Fatalf("expected completed progress line, got %q", output)
	}
	tokenIndex := strings.Index(output, "Token 消耗: 本次 575.5k")
	if tokenIndex < 0 {
		t.Fatalf("expected deferred console output, got %q", output)
	}
	if tokenIndex < completedIndex {
		t.Fatalf("expected console output after completed progress line, got %q", output)
	}
}

func TestPrintConsoleLinePreservesActiveProgressLine(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := New(2)
		tracker.enabled = true
		tracker.done = 1

		tracker.StartStep("写入技能文件")
		PrintConsoleLine("Token 消耗: 本次 120.1k")
		tracker.CompleteStep("写入技能文件")
	})

	if !strings.Contains(output, "[############################] 2/2") {
		t.Fatalf("expected completed progress line to still be printed, got %q", output)
	}
	completedIndex := strings.Index(output, "[############################] 2/2")
	tokenIndex := strings.Index(output, "Token 消耗: 本次 120.1k")
	if tokenIndex < completedIndex {
		t.Fatalf("expected console output after completed progress line, got %q", output)
	}
}

func TestPrintConsoleLineNowClearsActiveProgressLine(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := New(1)
		tracker.enabled = true

		tracker.StartStep("写入技能文件")
		PrintConsoleLineNow("错误: 生成失败")
		tracker.FailStep("写入技能文件")
	})

	if !strings.Contains(output, "\r\x1b[2K错误: 生成失败") {
		t.Fatalf("expected immediate console output to clear active progress line, got %q", output)
	}
}

func TestUpdateStepRefreshesActiveProgressLabel(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := New(1)
		tracker.enabled = true

		tracker.StartStep("分析当前代码库")
		tracker.UpdateStep("分析当前代码库（API Error: 529，本次调用 3m37s，15s 后重试）")
		tracker.CompleteStep("分析当前代码库")
	})

	if !strings.Contains(output, "分析当前代码库（API Error: 529，本次调用 3m37s，15s 后重试）") {
		t.Fatalf("expected updated progress label, got %q", output)
	}
}

func TestUpdateStepPrintsDetailWhenProgressDisabled(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := New(1)
		tracker.enabled = false

		tracker.UpdateStep("分析当前代码库 · 单元 2/17 · registry-management")
	})

	require.Contains(t, output, "分析当前代码库 · 单元 2/17 · registry-management")
}

func TestPrintConsoleLineAfterProgressPrintsAfterCompletedStep(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := New(1)
		tracker.enabled = true

		tracker.StartStep("分析当前代码库")
		PrintConsoleLineAfterProgress("Token 消耗: 本次 575.5k")
		tracker.CompleteStep("分析当前代码库")
	})

	completedIndex := strings.Index(output, "[############################] 1/1   分析当前代码库")
	if completedIndex < 0 {
		t.Fatalf("expected completed progress line before deferred console output, got %q", output)
	}
	tokenIndex := strings.Index(output, "Token 消耗: 本次 575.5k")
	if tokenIndex < 0 {
		t.Fatalf("expected deferred token usage output, got %q", output)
	}
	if tokenIndex < completedIndex {
		t.Fatalf("expected deferred token usage output after completed progress line, got %q", output)
	}
}

func TestPrintConsoleLineAfterProgressPrintsImmediatelyWithoutActiveStep(t *testing.T) {
	output := captureStdout(t, func() {
		PrintConsoleLineAfterProgress("Token 消耗: 本次 120.1k")
	})

	if output != "Token 消耗: 本次 120.1k\n" {
		t.Fatalf("expected console output immediately without active progress step, got %q", output)
	}
}

func TestPauseAfterFastStepUsesSharedFastStepPause(t *testing.T) {
	var slept []time.Duration

	PauseAfterFastStep(time.Now(), func(duration time.Duration) {
		slept = append(slept, duration)
	})

	require.Equal(t, []time.Duration{FastStepPause}, slept)
	require.Equal(t, 200*time.Millisecond, FastStepPause)
}

func TestPauseAfterFastStepDoesNotSleepAfterSlowStep(t *testing.T) {
	var slept []time.Duration

	PauseAfterFastStep(time.Now().Add(-FastStepPause-time.Millisecond), func(duration time.Duration) {
		slept = append(slept, duration)
	})

	require.Empty(t, slept)
}

func TestMultiTrackerRendersAggregateCountAndTaskLines(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := NewMulti([]string{"backend", "front"})
		tracker.enabled = true
		tracker.SetLabel("学习工作区子项目")
		tracker.SetTaskTotal(5)

		tracker.Start("backend", "分析当前代码库")
		tracker.Start("front", "检测增量文件变化")

		tracker.mu.Lock()
		tracker.tasks["backend"].startedAt = time.Now().Add(-2 * time.Second)
		tracker.tasks["front"].startedAt = time.Now().Add(-1 * time.Second)
		tracker.mu.Unlock()

		tracker.Render()
		tracker.Complete("front", "完成")
		tracker.Complete("backend", "完成")
	})

	if !strings.Contains(output, "backend") || !strings.Contains(output, "front") {
		t.Fatalf("expected output to include both task lines, got %q", output)
	}
	if !strings.Contains(output, "分析当前代码库") || !strings.Contains(output, "检测增量文件变化") {
		t.Fatalf("expected output to include each task status, got %q", output)
	}
	if !strings.Contains(output, "1/2 学习工作区子项目") || !strings.Contains(output, "2/2 学习工作区子项目") {
		t.Fatalf("expected aggregate completion counters, got %q", output)
	}
	if strings.Contains(output, "0/2 | backend") || strings.Contains(output, "0/2 | front") {
		t.Fatalf("expected task lines not to include aggregate counters, got %q", output)
	}
}

func TestMultiTrackerRendersPerTaskStepCounts(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := NewMulti([]string{"backend"})
		tracker.enabled = true
		tracker.SetLabel("学习工作区子项目")
		tracker.SetTaskTotal(5)

		tracker.Start("backend", "准备项目上下文")
		tracker.CompleteStep("backend", "准备项目上下文")
		tracker.Start("backend", "检测增量文件变化")
		tracker.CompleteStep("backend", "检测增量文件变化")
		tracker.Start("backend", "分析当前代码库")
	})

	if !strings.Contains(output, "backend      3/5") {
		t.Fatalf("expected child task line to include per-project step count, got %q", output)
	}
	if !strings.Contains(output, "分析当前代码库") {
		t.Fatalf("expected child task line to include current step label, got %q", output)
	}
}

func TestMultiTrackerAlignsPerTaskProgressPanel(t *testing.T) {
	tracker := NewMulti([]string{"agent", "cluster-manage"})
	tracker.enabled = true
	tracker.SetLabel("学习工作区子项目")
	tracker.SetTaskTotal(5)
	tracker.Start("agent", "检测增量文件变化")
	tracker.Start("cluster-manage", "分析当前代码库")

	tracker.mu.Lock()
	lines := tracker.renderLinesLocked()
	tracker.mu.Unlock()

	require.Len(t, lines, 3)
	require.Contains(t, lines[1], "agent")
	require.Contains(t, lines[2], "cluster-manage")
	firstBar := strings.Index(lines[1], "[")
	secondBar := strings.Index(lines[2], "[")
	require.NotEqual(t, -1, firstBar)
	require.Equal(t, firstBar, secondBar)
	firstStep := strings.Index(lines[1], "1/5")
	secondStep := strings.Index(lines[2], "1/5")
	require.NotEqual(t, -1, firstStep)
	require.Equal(t, firstStep, secondStep)
}

func TestMultiTrackerRenderKeepsCursorInsideProgressBlock(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := NewMulti([]string{"backend", "front"})
		tracker.enabled = true
		tracker.SetLabel("学习工作区子项目")
		tracker.SetTaskTotal(5)

		tracker.mu.Lock()
		tracker.tasks["backend"].label = "分析当前代码库"
		tracker.tasks["backend"].active = true
		tracker.tasks["backend"].startedAt = time.Now()
		tracker.tasks["front"].label = "分析当前代码库"
		tracker.tasks["front"].active = true
		tracker.tasks["front"].startedAt = time.Now()
		tracker.mu.Unlock()

		tracker.Render()
		tracker.Update("front", "分析当前代码库（API Error: 529，本次调用 3m37s，15s 后重试）")
	})

	if strings.HasSuffix(output, "\n") {
		t.Fatalf("expected active multi progress render to keep cursor on progress block without trailing newline, got %q", output)
	}
	if !strings.Contains(output, "\x1b[2F") {
		t.Fatalf("expected second render to move from last progress line back to the first line, got %q", output)
	}
	if strings.Contains(output, "\x1b[3F") {
		t.Fatalf("expected second render not to move above the active progress block, got %q", output)
	}
}

func TestMultiTrackerRenderMovesByWrappedPhysicalLines(t *testing.T) {
	restore := setTerminalWidthForTest(80)
	defer restore()

	output := captureStdout(t, func() {
		tracker := NewMulti([]string{"backend", "front"})
		tracker.enabled = true
		tracker.SetLabel("学习工作区子项目")
		tracker.SetTaskTotal(5)

		tracker.Start("backend", "分析当前代码库")
		tracker.Start("front", "分析当前代码库")
		tracker.Update("front", "分析当前代码库（API Error: 529 {\"error\":{\"code\":\"1305\",\"message\":\"[1305][该模型当前访问量过大，请您稍后再试][2026060311061203164c3724b3416a]\",\"type\":\"overloaded_error\"},\"request_id\":\"2026060311061203164c3724b3416a\",\"type\":\"error\"}，本次调用 3m37s，15s 后重试）")
		tracker.Render()
	})

	if !strings.Contains(output, "\x1b[5F") && !strings.Contains(output, "\x1b[6F") {
		t.Fatalf("expected wrapped progress render to move back across wrapped physical lines, got %q", output)
	}
	require.Contains(t, output, "\x1b[J")
	require.Contains(t, output, "request_id")
}

func TestMultiTrackerCompletedStepStillAnimatesUntilNextStep(t *testing.T) {
	tracker := NewMulti([]string{"backend"})
	tracker.enabled = true
	tracker.SetTaskTotal(5)

	tracker.Start("backend", "准备项目上下文")
	tracker.CompleteStep("backend", "准备项目上下文")

	tracker.mu.Lock()
	task := tracker.tasks["backend"]
	task.frame = 1
	line := formatMultiTaskLineWithTotal(task, tracker.width, tracker.taskTotal, tracker.nameWidth)
	tracker.mu.Unlock()

	if !strings.Contains(line, "/ backend") {
		t.Fatalf("expected completed intermediate step to keep spinner frame, got %q", line)
	}
	if !strings.Contains(line, "1/5") {
		t.Fatalf("expected completed intermediate step to keep completed step count, got %q", line)
	}
}

func TestMultiTrackerFailMarksTaskAndFlushesPendingLines(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := NewMulti([]string{"backend", "front"})
		tracker.enabled = true
		tracker.SetLabel("学习工作区子项目")

		tracker.Start("backend", "分析当前代码库")
		tracker.Start("front", "分析并保存项目画像")
		PrintConsoleLineAfterProgress("Token 消耗: 子项目 backend")
		tracker.Complete("backend", "完成")
		tracker.Fail("front", "失败")
	})

	if !strings.Contains(output, "front        失败") {
		t.Fatalf("expected failed task line, got %q", output)
	}
	if !strings.Contains(output, "2/2 学习工作区子项目") {
		t.Fatalf("expected aggregate progress to finish after failure, got %q", output)
	}
	if !strings.Contains(output, "Token 消耗: 子项目 backend") {
		t.Fatalf("expected pending console lines to flush after failure, got %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	resetConsoleState()
	defer resetConsoleState()

	tempFile, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create temp stdout: %v", err)
	}

	originalStdout := os.Stdout
	os.Stdout = tempFile
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := tempFile.Close(); err != nil {
		t.Fatalf("close temp stdout: %v", err)
	}

	data, err := os.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(data)
}

func resetConsoleState() {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	progressActive = false
	progressLineOpen = false
	pendingConsoleLines = nil
}

func setTerminalWidthForTest(width int) func() {
	original := terminalWidth
	terminalWidth = func() int {
		return width
	}
	return func() {
		terminalWidth = original
	}
}
