package logger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInitWithRetentionRemovesOldLogFiles(t *testing.T) {
	logDir := t.TempDir()

	for i := 0; i < 3; i++ {
		path := filepath.Join(logDir, fmt.Sprintf("old-%d.log", i))
		require.NoError(t, os.WriteFile(path, []byte("old"), 0644))
		require.NoError(t, os.Chtimes(path, time.Now().Add(time.Duration(-10+i)*time.Hour), time.Now().Add(time.Duration(-10+i)*time.Hour)))
	}

	require.NoError(t, InitWithRetention(logDir, "check", INFO, 2))
	defer Close()

	matches, err := filepath.Glob(filepath.Join(logDir, "*.log"))
	require.NoError(t, err)
	require.Len(t, matches, 2)
	require.FileExists(t, CurrentLogPath())
}

func TestWithScopedLogCapturesWorkerLogsAndFinalError(t *testing.T) {
	logDir := t.TempDir()
	wantErr := errors.New("duplicate dropped candidate")
	var logPath string

	err := WithScopedLog(context.Background(), logDir, "learn", INFO, 0, func(ctx context.Context, path string) error {
		logPath = path
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer BindScope(ctx)()
			Diagnostic("worker marker")
		}()
		<-done
		return wantErr
	})

	require.ErrorIs(t, err, wantErr)
	content, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	text := string(content)
	require.Contains(t, text, "worker marker")
	require.Contains(t, text, wantErr.Error())
	require.Contains(t, strings.ToLower(text), "error")
}
