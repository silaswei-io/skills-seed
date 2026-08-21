package container

import (
	"context"
	"sync"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
)

type patternStore interface {
	domain.PatternRepository
	domain.PatternStatsRepository
	domain.FileAnalysisTracker
	Close() error
}

type lazyPatternStore struct {
	dbPath string

	mu   sync.Mutex
	repo *boltdb.PatternRepository
	err  error
}

func newLazyPatternStore(dbPath string) patternStore {
	return &lazyPatternStore{dbPath: dbPath}
}

func (s *lazyPatternStore) Get(ctx context.Context, id string) (*domain.Pattern, error) {
	repo, err := s.ensure()
	if err != nil {
		return nil, err
	}
	return repo.Get(ctx, id)
}

func (s *lazyPatternStore) GetAll(ctx context.Context) ([]domain.Pattern, error) {
	repo, err := s.ensure()
	if err != nil {
		return nil, err
	}
	return repo.GetAll(ctx)
}

func (s *lazyPatternStore) GetByCategory(ctx context.Context, category domain.Category) ([]domain.Pattern, error) {
	repo, err := s.ensure()
	if err != nil {
		return nil, err
	}
	return repo.GetByCategory(ctx, category)
}

func (s *lazyPatternStore) GetHighConfidence(ctx context.Context, threshold float64) ([]domain.Pattern, error) {
	repo, err := s.ensure()
	if err != nil {
		return nil, err
	}
	return repo.GetHighConfidence(ctx, threshold)
}

func (s *lazyPatternStore) Save(ctx context.Context, p *domain.Pattern) error {
	repo, err := s.ensure()
	if err != nil {
		return err
	}
	return repo.Save(ctx, p)
}

func (s *lazyPatternStore) ApplyPatternMutation(ctx context.Context, mutation domain.PatternMutation) error {
	repo, err := s.ensure()
	if err != nil {
		return err
	}
	return repo.ApplyPatternMutation(ctx, mutation)
}

func (s *lazyPatternStore) FindSimilar(ctx context.Context, pattern *domain.Pattern) (*domain.Pattern, error) {
	repo, err := s.ensure()
	if err != nil {
		return nil, err
	}
	return repo.FindSimilar(ctx, pattern)
}

func (s *lazyPatternStore) Delete(ctx context.Context, id string) error {
	repo, err := s.ensure()
	if err != nil {
		return err
	}
	return repo.Delete(ctx, id)
}

func (s *lazyPatternStore) Count(ctx context.Context) (int, error) {
	repo, err := s.ensure()
	if err != nil {
		return 0, err
	}
	return repo.Count(ctx)
}

func (s *lazyPatternStore) GetPatternStats(ctx context.Context) ([]domain.PatternStats, error) {
	repo, err := s.ensure()
	if err != nil {
		return nil, err
	}
	return repo.GetPatternStats(ctx)
}

func (s *lazyPatternStore) GetAnalyzedFile(ctx context.Context, scope domain.FileAnalysisScope, path string) (*domain.FileAnalysisRecord, error) {
	repo, err := s.ensure()
	if err != nil {
		return nil, err
	}
	return repo.GetAnalyzedFile(ctx, scope, path)
}

func (s *lazyPatternStore) ListAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope) ([]domain.FileAnalysisRecord, error) {
	repo, err := s.ensure()
	if err != nil {
		return nil, err
	}
	return repo.ListAnalyzedFiles(ctx, scope)
}

func (s *lazyPatternStore) SaveAnalyzedFiles(ctx context.Context, records []domain.FileAnalysisRecord) error {
	repo, err := s.ensure()
	if err != nil {
		return err
	}
	return repo.SaveAnalyzedFiles(ctx, records)
}

func (s *lazyPatternStore) DeleteAnalyzedFiles(ctx context.Context, scope domain.FileAnalysisScope, paths []string) error {
	repo, err := s.ensure()
	if err != nil {
		return err
	}
	return repo.DeleteAnalyzedFiles(ctx, scope, paths)
}

func (s *lazyPatternStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repo == nil {
		return nil
	}
	err := s.repo.Close()
	s.repo = nil
	return err
}

func (s *lazyPatternStore) ensure() (*boltdb.PatternRepository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	if s.repo != nil {
		return s.repo, nil
	}
	repo, err := boltdb.NewPatternRepository(s.dbPath)
	if err != nil {
		s.err = patternRepositoryError(err)
		return nil, s.err
	}
	s.repo = repo
	return repo, nil
}
