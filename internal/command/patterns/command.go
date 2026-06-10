package patterns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/silaswei-io/skills-seed/internal/pkg/progress"
	"github.com/silaswei-io/skills-seed/internal/service/curator"
	"github.com/spf13/cobra"
)

// Cmd 返回 patterns 命令
func Cmd(cont *container.Container) *cobra.Command {
	patternsCmd := &cobra.Command{
		Use:     "patterns",
		Short:   i18n.Get("PatternsShort"),
		Long:    i18n.Get("PatternsLongDesc"),
		Example: i18n.Get("PatternsExample"),
	}

	patternsCmd.AddCommand(addCmd(cont))
	patternsCmd.AddCommand(compactCmd(cont))
	patternsCmd.AddCommand(statsCmd(cont))
	patternsCmd.AddCommand(showCmd(cont))
	return patternsCmd
}

func addCmd(cont *container.Container) *cobra.Command {
	var category string
	var files []string
	var userContext string

	cmd := &cobra.Command{
		Use:     "add <description>",
		Short:   i18n.Get("PatternsAddShort"),
		Long:    i18n.Get("PatternsAddLongDesc"),
		Example: i18n.Get("PatternsAddExample"),
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			if err := commandutil.RequireAgentAvailable(cont); err != nil {
				return err
			}

			description := strings.Join(args, " ")
			req := &agent.UserDefinePatternRequest{
				Description: description,
				Category:    category,
				Files:       files,
				UserContext: userContext,
				WorkDir:     cont.Config.Project.RootPath,
				Language:    cont.Config.Project.Language,
			}

			tracker := progress.New(1)
			retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
			ctx := retryProgress.WithContext(cmd.Context())
			label := i18n.Get("ProgressUserDefinePatternAI")
			var result *agent.UserDefinePatternResult
			err := tracker.RunStep(label, func() error {
				retryProgress.StartStep(label)
				var callErr error
				result, callErr = cont.Agent.UserDefinePattern(ctx, req)
				retryProgress.FinishStep(label, callErr == nil)
				return callErr
			})
			if err != nil {
				return err
			}
			if cont.CuratorSvc == nil {
				return fmt.Errorf("pattern curator is not configured")
			}

			curateTracker := progress.New(1)
			curated, err := cont.CuratorSvc.CurateAndStoreWithHooks(ctx, curator.CurateRequest{
				Operation:  curator.OperationUserDefined,
				Candidates: []domain.Pattern{*result.Pattern},
			}, curatorProgressHooks(curateTracker))
			if err != nil {
				return err
			}
			if len(curated.Written) == 0 {
				reason := "pattern was not written"
				if len(curated.Dropped) > 0 && curated.Dropped[0].Reason != "" {
					reason = curated.Dropped[0].Reason
				}
				return fmt.Errorf("%s", reason)
			}
			written := curated.Written[0]
			result.Pattern = &written

			logger.Info(i18n.GetWithParams("PatternsAddComplete", map[string]interface{}{
				"PatternID":   result.Pattern.ID,
				"PatternName": result.Pattern.Name,
				"Category":    string(result.Pattern.Category),
				"Source":      string(result.Pattern.Source),
			}))
			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", i18n.Get("PatternsAddFlagCategory"))
	cmd.Flags().StringArrayVarP(&files, "files", "f", nil, i18n.Get("PatternsAddFlagFiles"))
	cmd.Flags().StringVar(&userContext, "context", "", i18n.Get("PatternsAddFlagContext"))
	return cmd
}

func statsCmd(cont *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stats",
		Short:   "Show learned pattern quality and check hit statistics",
		Long:    "Show learned pattern quality metrics and check hit statistics, including specificity, confidence, effective score, hit count, and last hit time.",
		Example: "skills-seed patterns stats",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil || cont.PatternStats == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			stats, err := cont.PatternStats.GetPatternHitStats(context.Background())
			if err != nil {
				return err
			}
			return writeStats(cmd.OutOrStdout(), stats)
		},
	}
	return cmd
}

func showCmd(cont *container.Container) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "show [pattern-id]",
		Short:   "Show learned pattern database fields",
		Long:    "Show learned patterns with stored database fields such as source, timestamps, and code location metadata.",
		Example: "skills-seed patterns show\nskills-seed patterns show business-create-order\nskills-seed patterns show business-create-order --format json",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := requirePatternRepository(cont)
			if err != nil {
				return err
			}
			if format != "" && format != "table" && format != "json" {
				return fmt.Errorf("unsupported format %q", format)
			}
			if len(args) == 1 {
				pattern, err := repo.Get(context.Background(), args[0])
				if err != nil {
					return err
				}
				pattern.NormalizeAfterLoad()
				if format == "json" {
					return writePatternJSON(cmd.OutOrStdout(), pattern)
				}
				return writePatternDetails(cmd.OutOrStdout(), pattern)
			}
			patterns, err := repo.GetAll(context.Background())
			if err != nil {
				return err
			}
			for i := range patterns {
				patterns[i].NormalizeAfterLoad()
			}
			if format == "json" {
				return writePatternsJSON(cmd.OutOrStdout(), patterns)
			}
			return writePatternList(cmd.OutOrStdout(), patterns)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table or json")
	return cmd
}

func requirePatternRepository(cont *container.Container) (domain.PatternRepository, error) {
	if cont == nil {
		return nil, fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
	}
	if cont.PatternReader != nil {
		return cont.PatternReader, nil
	}
	if cont.PatternRepo != nil {
		return cont.PatternRepo, nil
	}
	return nil, fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
}

func writeStats(w io.Writer, stats []domain.PatternHitStats) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PATTERN\tCATEGORY\tSPECIFICITY\tCONFIDENCE\tEFFECTIVE\tHITS\tLAST_HIT"); err != nil {
		return err
	}
	for _, stat := range stats {
		lastHit := "-"
		if !stat.LastHitAt.IsZero() {
			lastHit = stat.LastHitAt.Format("2006-01-02 15:04:05")
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%.2f\t%.2f\t%.2f\t%d\t%s\n",
			stat.Pattern.ID,
			stat.Pattern.Category,
			stat.Pattern.Metrics.SpecificityScore,
			stat.Pattern.Confidence,
			stat.Pattern.Metrics.EffectiveScore,
			stat.HitCount,
			lastHit,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writePatternList(w io.Writer, patterns []domain.Pattern) error {
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].Category != patterns[j].Category {
			return patterns[i].Category < patterns[j].Category
		}
		return patterns[i].ID < patterns[j].ID
	})

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tCATEGORY\tSOURCE\tCONFIDENCE\tCREATED_AT\tUPDATED_AT\tLOCATION_STATUS\tCURRENT_LOCATION"); err != nil {
		return err
	}
	for _, pattern := range patterns {
		status, location := patternLocationSummary(pattern)
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%.2f\t%s\t%s\t%s\t%s\n",
			pattern.ID,
			pattern.Category,
			pattern.Source,
			pattern.Confidence,
			formatTime(pattern.CreatedAt),
			formatTime(pattern.UpdatedAt),
			status,
			location,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writePatternDetails(w io.Writer, pattern *domain.Pattern) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fields := []struct {
		key   string
		value string
	}{
		{"id", pattern.ID},
		{"name", pattern.Name},
		{"category", string(pattern.Category)},
		{"source", string(pattern.Source)},
		{"confidence", fmt.Sprintf("%.2f", pattern.Confidence)},
		{"frequency", fmt.Sprintf("%d", pattern.Frequency)},
		{"created_at", formatTime(pattern.CreatedAt)},
		{"updated_at", formatTime(pattern.UpdatedAt)},
		{"description", pattern.Description},
		{"rule", pattern.Rule},
	}
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", field.key, field.value); err != nil {
			return err
		}
	}

	if pattern.BusinessMethod != nil {
		method := pattern.BusinessMethod
		location := method.CodeLocation
		methodFields := []struct {
			key   string
			value string
		}{
			{"business_method", method.Name},
			{"current_location", location.CurrentLocation},
			{"historical_location", location.HistoricalLocation},
			{"location_status", string(location.Status)},
			{"change_kinds", joinChangeKinds(location.ChangeKinds)},
			{"location_confidence", formatOptionalFloat(location.Confidence)},
			{"location_verified_at", formatTime(location.VerifiedAt)},
			{"location_created_at", formatTime(location.CreatedAt)},
			{"location_updated_at", formatTime(location.UpdatedAt)},
		}
		for _, field := range methodFields {
			if field.value == "" || field.value == "-" {
				continue
			}
			if _, err := fmt.Fprintf(tw, "%s\t%s\n", field.key, field.value); err != nil {
				return err
			}
		}
		if location.Snapshot != nil {
			snapshotFields := []struct {
				key   string
				value string
			}{
				{"snapshot_language", location.Snapshot.Language},
				{"snapshot_kind", location.Snapshot.Kind},
				{"snapshot_namespace", location.Snapshot.Namespace},
				{"snapshot_receiver", location.Snapshot.Receiver},
				{"snapshot_name", location.Snapshot.Name},
				{"snapshot_signature_hash", location.Snapshot.SignatureHash},
				{"snapshot_inputs", strings.Join(location.Snapshot.InputTypes, ",")},
				{"snapshot_outputs", strings.Join(location.Snapshot.OutputTypes, ",")},
				{"snapshot_body_hash", location.Snapshot.BodyHash},
				{"snapshot_dependencies", strings.Join(location.Snapshot.DependencySymbols, ",")},
			}
			for _, field := range snapshotFields {
				if field.value == "" {
					continue
				}
				if _, err := fmt.Fprintf(tw, "%s\t%s\n", field.key, field.value); err != nil {
					return err
				}
			}
		}
	}
	return tw.Flush()
}

func writePatternJSON(w io.Writer, pattern *domain.Pattern) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(pattern)
}

func writePatternsJSON(w io.Writer, patterns []domain.Pattern) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(patterns)
}

func patternLocationSummary(pattern domain.Pattern) (string, string) {
	if pattern.BusinessMethod == nil {
		return "-", "-"
	}
	location := pattern.BusinessMethod.CodeLocation
	status := string(location.Status)
	if status == "" {
		status = "-"
	}
	current := pattern.BusinessMethod.DisplayLocation()
	if current == "" {
		current = "-"
	}
	return status, current
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

func joinChangeKinds(kinds []domain.CodeLocationChangeKind) string {
	if len(kinds) == 0 {
		return ""
	}
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, string(kind))
	}
	return strings.Join(parts, ",")
}

func formatOptionalFloat(value float64) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", value)
}

func compactCmd(cont *container.Container) *cobra.Command {
	var category string
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "compact",
		Short:   i18n.Get("PatternsCompactShort"),
		Long:    i18n.Get("PatternsCompactLongDesc"),
		Example: i18n.Get("PatternsCompactExample"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			if err := commandutil.RequireAgentAvailable(cont); err != nil {
				return err
			}
			if cont.CuratorSvc == nil {
				return fmt.Errorf("pattern curator is not configured")
			}
			tracker := progress.New(1)
			result, err := cont.CuratorSvc.CompactWithHooks(cmd.Context(), curator.CompactRequest{
				Category: category,
				DryRun:   dryRun,
			}, curatorProgressHooks(tracker))
			if err != nil {
				return err
			}
			logger.Info(i18n.GetWithParams("PatternsCompactComplete", map[string]interface{}{
				"TotalInput":   result.Summary.TotalCandidates,
				"TotalWritten": result.Summary.TotalWritten,
				"TotalDropped": result.Summary.TotalDropped,
				"MergeCount":   result.Summary.MergeCount,
			}))
			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", i18n.Get("PatternsCompactFlagCategory"))
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.Get("PatternsCompactFlagDryRun"))
	return cmd
}

func curatorProgressHooks(tracker *progress.Tracker) curator.ProgressHooks {
	return curator.ProgressHooks{
		OnStepStart:    tracker.StartStep,
		OnStepUpdate:   tracker.UpdateStep,
		OnStepComplete: tracker.CompleteStep,
	}
}
