package logger

import (
	"fmt"
	"os"
	"path/filepath"
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
