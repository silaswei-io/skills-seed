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

func TestPrintConsoleLineBreaksActiveProgressLine(t *testing.T) {
	output := captureStdout(t, func() {
		tracker := New(1)
		tracker.enabled = true

		tracker.StartStep("分析当前代码库")
		PrintConsoleLine("Token 消耗: 本次 575.5k")
		tracker.CompleteStep("分析当前代码库")
	})

	if !strings.Contains(output, "分析当前代码库\nToken 消耗") {
		t.Fatalf("expected console message to start on a new line after active progress output, got %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

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
