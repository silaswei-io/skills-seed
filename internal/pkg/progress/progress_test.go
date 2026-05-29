package progress

import (
	"os"
	"strings"
	"testing"
	"time"
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
