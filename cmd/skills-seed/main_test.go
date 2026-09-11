package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecute(t *testing.T) {
	var stderr bytes.Buffer
	require.NoError(t, execute(func() error { return nil }, &stderr))
	require.Empty(t, stderr.String())

	wantErr := errors.New("startup failed")
	err := execute(func() error { return wantErr }, &stderr)
	require.ErrorIs(t, err, wantErr)
	require.Contains(t, stderr.String(), "startup failed")
}
