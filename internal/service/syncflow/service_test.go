package syncflow

import (
	"context"
	"errors"
	"testing"

	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/stretchr/testify/require"
)

type changeRecorderStub struct {
	details []string
}

func (r *changeRecorderStub) Detail(message string) {
	r.details = append(r.details, message)
}

func TestServiceRunValidatesDependencies(t *testing.T) {
	_, err := (Service{}).Run(context.Background(), Request{})
	require.ErrorContains(t, err, "learn dependency")

	_, err = (Service{LearnCurrent: func(context.Context, LearnCurrentRequest) (domain.LearnCurrentResult, error) {
		return domain.LearnCurrentResult{}, nil
	}}).Run(context.Background(), Request{})
	require.ErrorContains(t, err, "generate dependency")
}

func TestServiceRunPropagatesLearnFailure(t *testing.T) {
	wantErr := errors.New("learn failed")
	service := Service{
		LearnCurrent: func(context.Context, LearnCurrentRequest) (domain.LearnCurrentResult, error) {
			return domain.LearnCurrentResult{}, wantErr
		},
		Generate: func(context.Context) error { return nil },
	}

	_, err := service.Run(context.Background(), Request{})

	require.ErrorIs(t, err, wantErr)
}

func TestServiceRunGeneratesWhenOutputIsMissing(t *testing.T) {
	want := domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{NoFileChanges: true}}
	wantRequest := LearnCurrentRequest{StateScope: "scope", UserContext: "context", Force: true}
	recorder := &changeRecorderStub{}
	generated := false
	service := Service{
		LearnCurrent: func(_ context.Context, req LearnCurrentRequest) (domain.LearnCurrentResult, error) {
			require.Equal(t, wantRequest, req)
			return want, nil
		},
		Generate: func(context.Context) error {
			generated = true
			return nil
		},
		OutputMissing: func() bool { return true },
	}

	got, err := service.Run(context.Background(), Request{Learn: wantRequest, Change: recorder})

	require.NoError(t, err)
	require.Equal(t, want, got)
	require.True(t, generated)
	require.Len(t, recorder.details, 2)
}

func TestServiceRunSkipsGenerationWithoutChanges(t *testing.T) {
	generated := false
	service := Service{
		LearnCurrent: func(context.Context, LearnCurrentRequest) (domain.LearnCurrentResult, error) {
			return domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{NoFileChanges: true}}, nil
		},
		Generate: func(context.Context) error {
			generated = true
			return nil
		},
	}

	_, err := service.Run(context.Background(), Request{})

	require.NoError(t, err)
	require.False(t, generated)
}

func TestRunAfterLearnPropagatesGenerateFailure(t *testing.T) {
	wantErr := errors.New("generate failed")
	err := RunAfterLearn(
		domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{ChangedFiles: 1}},
		false,
		func() error { return wantErr },
		nil,
	)

	require.ErrorIs(t, err, wantErr)
}

func TestShouldGenerateAfterLearn(t *testing.T) {
	tests := []struct {
		name    string
		summary domain.LearnCurrentSummary
		want    bool
	}{
		{name: "workspace unchanged", summary: domain.LearnCurrentSummary{Projects: 2}, want: false},
		{name: "workspace project changed", summary: domain.LearnCurrentSummary{Projects: 2, ChangedProjects: 1}, want: true},
		{name: "workspace relations changed", summary: domain.LearnCurrentSummary{Projects: 2, WorkspaceChanged: true}, want: true},
		{name: "explicit no file changes", summary: domain.LearnCurrentSummary{NoFileChanges: true}, want: false},
		{name: "changed file", summary: domain.LearnCurrentSummary{ChangedFiles: 1}, want: true},
		{name: "deleted file", summary: domain.LearnCurrentSummary{DeletedFiles: 1}, want: true},
		{name: "pattern found", summary: domain.LearnCurrentSummary{PatternsFound: 1}, want: true},
		{name: "pattern saved", summary: domain.LearnCurrentSummary{PatternsSaved: 1}, want: true},
		{name: "empty project", summary: domain.LearnCurrentSummary{}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, ShouldGenerateAfterLearn(domain.LearnCurrentResult{Summary: test.summary}))
		})
	}
}

func TestRecordLearnSummaryCoversProjectAndWorkspaceResults(t *testing.T) {
	RecordLearnSummary(nil, domain.LearnCurrentResult{})

	recorder := &changeRecorderStub{}
	RecordLearnSummary(recorder, domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{
		Projects: 2, ChangedProjects: 1, WorkspaceChanged: true,
	}})
	require.Len(t, recorder.details, 2)

	recorder.details = nil
	RecordLearnSummary(recorder, domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{Projects: 2}})
	require.Len(t, recorder.details, 1)

	recorder.details = nil
	RecordLearnSummary(recorder, domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{NoFileChanges: true}})
	require.Len(t, recorder.details, 1)

	recorder.details = nil
	RecordLearnSummary(recorder, domain.LearnCurrentResult{Summary: domain.LearnCurrentSummary{ChangedFiles: 1}})
	require.Len(t, recorder.details, 1)
}
