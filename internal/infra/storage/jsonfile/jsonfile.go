package jsonfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/silaswei-io/skills-seed/internal/infra/storage/fileio"
)

// Labels contains user-facing error prefixes for JSON file persistence.
type Labels struct {
	Read      string
	Parse     string
	CreateDir string
	Marshal   string
	Write     string
}

// Store persists one JSON document at a fixed path.
type Store[T any] struct {
	Path     string
	NotFound error
	NilValue error
	Labels   Labels
}

// Get reads and unmarshals the store path.
func (s Store[T]) Get(ctx context.Context) (*T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && s.NotFound != nil {
			return nil, s.NotFound
		}
		return nil, wrapLabel(s.Labels.Read, err)
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, wrapLabel(s.Labels.Parse, err)
	}
	return &value, nil
}

// Save marshals value as indented JSON and writes it to the store path.
func (s Store[T]) Save(ctx context.Context, value *T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if value == nil {
		if s.NilValue != nil {
			return s.NilValue
		}
		return fmt.Errorf("json value is nil")
	}

	if err := os.MkdirAll(filepath.Dir(s.Path), 0755); err != nil {
		return wrapLabel(s.Labels.CreateDir, err)
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return wrapLabel(s.Labels.Marshal, err)
	}
	data = append(data, '\n')

	if err := fileio.WriteFileAtomic(s.Path, data, 0644); err != nil {
		return wrapLabel(s.Labels.Write, err)
	}
	return nil
}

func wrapLabel(label string, err error) error {
	if label == "" {
		return err
	}
	return fmt.Errorf("%s: %w", label, err)
}
