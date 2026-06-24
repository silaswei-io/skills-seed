package patterns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/silaswei-io/skills-seed/internal/agent"
	"github.com/silaswei-io/skills-seed/internal/command/commandutil"
	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/infra/config"
	"github.com/silaswei-io/skills-seed/internal/infra/storage/boltdb"
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
	patternsCmd.AddCommand(deleteCmd(cont))
	patternsCmd.AddCommand(statsCmd(cont))
	patternsCmd.AddCommand(showCmd(cont))
	return patternsCmd
}

func addCmd(cont *container.Container) *cobra.Command {
	var category string
	var files []string

	cmd := &cobra.Command{
		Use:     "add <description>",
		Short:   i18n.Get("PatternsAddShort"),
		Long:    i18n.Get("PatternsAddLongDesc"),
		Example: i18n.Get("PatternsAddExample"),
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			if err := commandutil.RequireAgentAvailable(cont); err != nil {
				return err
			}

			description, err := resolveAddPatternDescription(args)
			if err != nil {
				return err
			}
			req := &agent.UserDefinePatternRequest{
				Description: description,
				Category:    category,
				Files:       files,
				WorkDir:     cont.Config.Project.RootPath,
				Language:    cont.Config.Project.Language,
			}

			tracker := progress.New(1)
			retryProgress := agent.NewRetryProgressBinder(tracker.UpdateStep)
			ctx := retryProgress.WithContext(cmd.Context())
			label := i18n.Get("ProgressUserDefinePatternAI")
			var result *agent.UserDefinePatternResult
			err = tracker.RunStep(label, func() error {
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

			written, err := StoreUserDefinedPattern(ctx, cont, description, *result.Pattern)
			if err != nil {
				return err
			}
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
	return cmd
}

type workspacePatternTargetPlan struct {
	Workspace bool
	Projects  []string
}

func resolveAddPatternDescription(args []string) (string, error) {
	description := strings.TrimSpace(strings.Join(args, " "))
	if description != "" {
		return description, nil
	}
	return "", fmt.Errorf("%s", i18n.Get("PatternsAddRequireDescription"))
}

// StoreUserDefinedPattern 保存用户定义模式，并按当前运行模式标记需要重新生成的 skills 目标。
func StoreUserDefinedPattern(ctx context.Context, cont *container.Container, description string, pattern domain.Pattern) (domain.Pattern, error) {
	if cont.ConfigRepo != nil && cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
		return storeWorkspaceUserDefinedPattern(ctx, cont, description, pattern)
	}
	written, err := curateAndStoreUserDefinedPattern(ctx, cont.CuratorSvc, pattern)
	if err != nil {
		return domain.Pattern{}, err
	}
	if cont.StateRepo != nil {
		if err := cont.StateRepo.MarkSkillsDirty(ctx, domain.SkillsDirtyTarget{Project: true}); err != nil {
			return domain.Pattern{}, err
		}
	}
	return written, nil
}

func storeWorkspaceUserDefinedPattern(ctx context.Context, cont *container.Container, description string, pattern domain.Pattern) (domain.Pattern, error) {
	projects := cont.ConfigRepo.GetWorkspaceConfig().Projects
	plan := planWorkspacePatternTargets(description, projects)
	rootPattern := pattern
	if len(plan.Projects) == 1 {
		rootPattern.ProjectID = plan.Projects[0]
		for _, project := range projects {
			if project.ID == plan.Projects[0] {
				rootPattern.ScopePath = project.Path
				rootPattern.WorkspaceRole = project.Type
				break
			}
		}
	}
	written, err := curateAndStoreUserDefinedPattern(ctx, cont.CuratorSvc, rootPattern)
	if err != nil {
		return domain.Pattern{}, err
	}
	for _, projectID := range plan.Projects {
		if err := storeWorkspaceChildPattern(ctx, cont, projectID, written); err != nil {
			return domain.Pattern{}, err
		}
	}
	if cont.StateRepo != nil {
		if err := cont.StateRepo.MarkSkillsDirty(ctx, domain.SkillsDirtyTarget{Workspace: plan.Workspace, Projects: plan.Projects}); err != nil {
			return domain.Pattern{}, err
		}
	}
	return written, nil
}

func curateAndStoreUserDefinedPattern(ctx context.Context, svc *curator.Service, pattern domain.Pattern) (domain.Pattern, error) {
	curateTracker := progress.New(1)
	curated, err := svc.CurateAndStoreWithHooks(ctx, curator.CurateRequest{
		Operation:  curator.OperationUserDefined,
		Candidates: []domain.Pattern{pattern},
	}, curatorProgressHooks(curateTracker))
	if err != nil {
		return domain.Pattern{}, err
	}
	if len(curated.Written) == 0 {
		reason := "pattern was not written"
		if len(curated.Dropped) > 0 && curated.Dropped[0].Reason != "" {
			reason = curated.Dropped[0].Reason
		}
		return domain.Pattern{}, fmt.Errorf("%s", reason)
	}
	return curated.Written[0], nil
}

func storeWorkspaceChildPattern(ctx context.Context, cont *container.Container, projectID string, pattern domain.Pattern) error {
	repo, err := openWorkspaceChildPatternRepo(cont, projectID)
	if err != nil {
		return err
	}
	if repo == nil {
		return nil
	}
	defer repo.Close()
	childPattern := pattern
	childPattern.ProjectID = ""
	childPattern.ScopePath = ""
	childPattern.WorkspaceRole = ""
	return repo.Save(ctx, &childPattern)
}

func deleteCmd(cont *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <pattern-id>",
		Aliases: []string{"rm", "remove"},
		Short:   i18n.Get("PatternsDeleteShort"),
		Long:    i18n.Get("PatternsDeleteLongDesc"),
		Example: i18n.Get("PatternsDeleteExample"),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("%s", i18n.Get("PatternsDeleteRequireID"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil || cont.PatternRepo == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			if err := DeletePattern(cmd.Context(), cont, args[0]); err != nil {
				return err
			}
			logger.Info(i18n.GetWithParams("PatternsDeleteComplete", map[string]interface{}{"PatternID": strings.TrimSpace(args[0])}))
			return nil
		},
	}
	return cmd
}

// DeletePattern 删除模式，并按当前运行模式标记需要重新生成的 skills 目标。
func DeletePattern(ctx context.Context, cont *container.Container, patternID string) error {
	patternID = strings.TrimSpace(patternID)
	if patternID == "" {
		return fmt.Errorf("%s", i18n.Get("PatternsDeleteRequireID"))
	}
	pattern, err := cont.PatternRepo.Get(ctx, patternID)
	if err != nil {
		return err
	}
	if err := cont.PatternRepo.Delete(ctx, patternID); err != nil {
		return err
	}
	if cont.ConfigRepo != nil && cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
		return deleteWorkspacePattern(ctx, cont, patternID, pattern)
	}
	if cont.StateRepo != nil {
		return cont.StateRepo.MarkSkillsDirty(ctx, domain.SkillsDirtyTarget{Project: true})
	}
	return nil
}

func deleteWorkspacePattern(ctx context.Context, cont *container.Container, patternID string, pattern *domain.Pattern) error {
	projects := []string{}
	if pattern != nil && strings.TrimSpace(pattern.ProjectID) != "" {
		projects = append(projects, strings.TrimSpace(pattern.ProjectID))
		if err := deleteWorkspaceChildPattern(ctx, cont, pattern.ProjectID, patternID); err != nil {
			return err
		}
	}
	if cont.StateRepo != nil {
		return cont.StateRepo.MarkSkillsDirty(ctx, domain.SkillsDirtyTarget{Workspace: true, Projects: projects})
	}
	return nil
}

func deleteWorkspaceChildPattern(ctx context.Context, cont *container.Container, projectID, patternID string) error {
	repo, err := openWorkspaceChildPatternRepo(cont, projectID)
	if err != nil {
		return err
	}
	if repo == nil {
		return nil
	}
	defer repo.Close()
	return repo.Delete(ctx, patternID)
}

func openWorkspaceChildPatternRepo(cont *container.Container, projectID string) (*boltdb.PatternRepository, error) {
	project, ok := workspaceProjectByID(cont.ConfigRepo.GetWorkspaceConfig().Projects, projectID)
	if !ok {
		return nil, nil
	}
	projectRoot := cont.ConfigRepo.GetProjectConfig().RootPath
	childRoot := filepath.Join(projectRoot, filepath.FromSlash(project.Path))
	return boltdb.NewPatternRepository(filepath.Join(childRoot, ".skills-seed", "memory", "project.db"))
}

func workspaceProjectByID(projects []config.WorkspaceProjectConfig, projectID string) (config.WorkspaceProjectConfig, bool) {
	for _, project := range projects {
		if project.ID == projectID {
			return project, true
		}
	}
	return config.WorkspaceProjectConfig{}, false
}

func planWorkspacePatternTargets(description string, projects []config.WorkspaceProjectConfig) workspacePatternTargetPlan {
	text := strings.ToLower(description)
	projectIDs := make([]string, 0)
	for _, project := range projects {
		if workspaceProjectMentioned(text, project) {
			projectIDs = append(projectIDs, project.ID)
		}
	}
	sort.Strings(projectIDs)
	return workspacePatternTargetPlan{Workspace: true, Projects: projectIDs}
}

func workspaceProjectMentioned(text string, project config.WorkspaceProjectConfig) bool {
	candidates := []string{project.ID, project.Path}
	for _, candidate := range candidates {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate != "" && strings.Contains(text, candidate) {
			return true
		}
	}
	return false
}

func statsCmd(cont *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stats",
		Short:   i18n.Get("PatternsStatsShort"),
		Long:    i18n.Get("PatternsStatsLongDesc"),
		Example: i18n.Get("PatternsStatsExample"),
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
		Short:   i18n.Get("PatternsShowShort"),
		Long:    i18n.Get("PatternsShowLongDesc"),
		Example: i18n.Get("PatternsShowExample"),
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := requirePatternRepository(cont)
			if err != nil {
				return err
			}
			if format != "" && format != "table" && format != "json" {
				return fmt.Errorf("%s", i18n.GetWithParams("PatternsShowUnsupportedFormat", map[string]interface{}{"Format": format}))
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
	cmd.Flags().StringVar(&format, "format", "table", i18n.Get("PatternsShowFlagFormat"))
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
	if _, err := fmt.Fprintf(
		tw,
		"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		i18n.Get("PatternsStatsHeaderPattern"),
		i18n.Get("PatternsStatsHeaderCategory"),
		i18n.Get("PatternsStatsHeaderSpecificity"),
		i18n.Get("PatternsStatsHeaderConfidence"),
		i18n.Get("PatternsStatsHeaderEffective"),
		i18n.Get("PatternsStatsHeaderHits"),
		i18n.Get("PatternsStatsHeaderLastHit"),
	); err != nil {
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
	if _, err := fmt.Fprintf(
		tw,
		"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		i18n.Get("PatternsShowHeaderID"),
		i18n.Get("PatternsShowHeaderCategory"),
		i18n.Get("PatternsShowHeaderSource"),
		i18n.Get("PatternsShowHeaderConfidence"),
		i18n.Get("PatternsShowHeaderCreatedAt"),
		i18n.Get("PatternsShowHeaderUpdatedAt"),
		i18n.Get("PatternsShowHeaderLocationStatus"),
		i18n.Get("PatternsShowHeaderCurrentLocation"),
	); err != nil {
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
		{i18n.Get("PatternsShowFieldID"), pattern.ID},
		{i18n.Get("PatternsShowFieldName"), pattern.Name},
		{i18n.Get("PatternsShowFieldCategory"), string(pattern.Category)},
		{i18n.Get("PatternsShowFieldSource"), string(pattern.Source)},
		{i18n.Get("PatternsShowFieldConfidence"), fmt.Sprintf("%.2f", pattern.Confidence)},
		{i18n.Get("PatternsShowFieldFrequency"), fmt.Sprintf("%d", pattern.Frequency)},
		{i18n.Get("PatternsShowFieldCreatedAt"), formatTime(pattern.CreatedAt)},
		{i18n.Get("PatternsShowFieldUpdatedAt"), formatTime(pattern.UpdatedAt)},
		{i18n.Get("PatternsShowFieldDescription"), pattern.Description},
		{i18n.Get("PatternsShowFieldRule"), pattern.Rule},
		{i18n.Get("PatternsShowFieldGoodExample"), pattern.GoodExample},
		{i18n.Get("PatternsShowFieldBadExample"), pattern.BadExample},
		{i18n.Get("PatternsShowFieldEvidenceLocation"), formatEvidenceLocations(pattern.EvidenceLocations)},
		{i18n.Get("PatternsShowFieldSpecificity"), formatOptionalFloat(pattern.Metrics.SpecificityScore)},
		{i18n.Get("PatternsShowFieldEvidenceCount"), formatOptionalInt(pattern.Metrics.EvidenceCount)},
		{i18n.Get("PatternsShowFieldGenericPenalty"), formatOptionalFloat(pattern.Metrics.GenericPenalty)},
		{i18n.Get("PatternsShowFieldEffectiveScore"), formatOptionalFloat(pattern.Metrics.EffectiveScore)},
		{i18n.Get("PatternsShowFieldMerged"), formatOptionalBool(pattern.Merged)},
		{i18n.Get("PatternsShowFieldMergedFrom"), strings.Join(pattern.MergedFrom, ",")},
		{i18n.Get("PatternsShowFieldGenerated"), formatOptionalBool(pattern.Generated)},
		{i18n.Get("PatternsShowFieldProjectID"), pattern.ProjectID},
		{i18n.Get("PatternsShowFieldScopePath"), pattern.ScopePath},
		{i18n.Get("PatternsShowFieldWorkspaceRole"), pattern.WorkspaceRole},
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
			{i18n.Get("PatternsShowFieldBusinessMethod"), method.Name},
			{i18n.Get("PatternsShowFieldBusinessDescription"), method.Description},
			{i18n.Get("PatternsShowFieldBusinessUsage"), method.Usage},
			{i18n.Get("PatternsShowFieldBusinessType"), method.Type},
			{i18n.Get("PatternsShowFieldBusinessFunction"), method.Function},
			{i18n.Get("PatternsShowFieldBusinessPrerequisites"), method.Prerequisites},
			{i18n.Get("PatternsShowFieldBusinessReturns"), method.Returns},
			{i18n.Get("PatternsShowFieldCurrentLocation"), location.CurrentLocation},
			{i18n.Get("PatternsShowFieldHistoricalLocation"), location.HistoricalLocation},
			{i18n.Get("PatternsShowFieldLocationStatus"), string(location.Status)},
			{i18n.Get("PatternsShowFieldChangeKinds"), joinChangeKinds(location.ChangeKinds)},
			{i18n.Get("PatternsShowFieldLocationConfidence"), formatOptionalFloat(location.Confidence)},
			{i18n.Get("PatternsShowFieldLocationVerifiedAt"), formatTime(location.VerifiedAt)},
			{i18n.Get("PatternsShowFieldLocationCreatedAt"), formatTime(location.CreatedAt)},
			{i18n.Get("PatternsShowFieldLocationUpdatedAt"), formatTime(location.UpdatedAt)},
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
				{i18n.Get("PatternsShowFieldSnapshotLanguage"), location.Snapshot.Language},
				{i18n.Get("PatternsShowFieldSnapshotKind"), location.Snapshot.Kind},
				{i18n.Get("PatternsShowFieldSnapshotNamespace"), location.Snapshot.Namespace},
				{i18n.Get("PatternsShowFieldSnapshotReceiver"), location.Snapshot.Receiver},
				{i18n.Get("PatternsShowFieldSnapshotName"), location.Snapshot.Name},
				{i18n.Get("PatternsShowFieldSnapshotSignatureHash"), location.Snapshot.SignatureHash},
				{i18n.Get("PatternsShowFieldSnapshotInputs"), strings.Join(location.Snapshot.InputTypes, ",")},
				{i18n.Get("PatternsShowFieldSnapshotOutputs"), strings.Join(location.Snapshot.OutputTypes, ",")},
				{i18n.Get("PatternsShowFieldSnapshotBodyHash"), location.Snapshot.BodyHash},
				{i18n.Get("PatternsShowFieldSnapshotDependencies"), strings.Join(location.Snapshot.DependencySymbols, ",")},
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
		for _, history := range formatLocationHistory(location.History) {
			if _, err := fmt.Fprintf(tw, "%s\t%s\n", i18n.Get("PatternsShowFieldLocationHistory"), history); err != nil {
				return err
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
	if pattern.BusinessMethod != nil {
		location := pattern.BusinessMethod.CodeLocation
		status := string(location.Status)
		if status == "" {
			status = "-"
		}
		current := pattern.BusinessMethod.DisplayLocation()
		if current != "" {
			return status, current
		}
	}
	if location := firstEvidenceDisplayLocation(pattern.EvidenceLocations); location != "" {
		return "evidence", location
	}
	return "-", "-"
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

func formatOptionalInt(value int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func formatOptionalBool(value bool) string {
	if !value {
		return ""
	}
	return fmt.Sprintf("%t", value)
}

func formatLocationHistory(history []domain.CodeLocationHistory) []string {
	lines := make([]string, 0, len(history))
	for _, item := range history {
		parts := []string{
			item.Location,
			string(item.Status),
			joinChangeKinds(item.ChangeKinds),
			formatTime(item.ChangedAt),
			item.Note,
		}
		trimmed := trimRightEmpty(parts)
		if len(trimmed) == 0 {
			continue
		}
		lines = append(lines, strings.Join(trimmed, " | "))
	}
	return lines
}

func formatEvidenceLocations(locations []domain.PatternEvidenceLocation) string {
	lines := make([]string, 0, len(locations))
	for _, location := range locations {
		display := location.DisplayLocation()
		if display == "" {
			continue
		}
		parts := []string{
			display,
			location.Kind,
			location.Symbol,
			formatOptionalFloat(location.Confidence),
			location.Description,
		}
		lines = append(lines, strings.Join(trimRightEmpty(parts), " | "))
	}
	return strings.Join(lines, "\n")
}

func firstEvidenceDisplayLocation(locations []domain.PatternEvidenceLocation) string {
	for _, location := range locations {
		if display := location.DisplayLocation(); display != "" {
			return display
		}
	}
	return ""
}

func trimRightEmpty(values []string) []string {
	end := len(values)
	for end > 0 {
		value := strings.TrimSpace(values[end-1])
		if value != "" && value != "-" {
			break
		}
		end--
	}
	return values[:end]
}

func compactCmd(cont *container.Container) *cobra.Command {
	var category string
	var dryRun bool
	var useAI bool

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
			if useAI {
				if err := commandutil.RequireAgentAvailable(cont); err != nil {
					return err
				}
			}
			if cont.CuratorSvc == nil {
				return fmt.Errorf("pattern curator is not configured")
			}
			tracker := progress.New(1)
			result, err := cont.CuratorSvc.CompactWithHooks(cmd.Context(), curator.CompactRequest{
				Category: category,
				DryRun:   dryRun,
				UseAI:    useAI,
			}, curatorProgressHooks(tracker))
			if err != nil {
				return err
			}
			if !dryRun && result.Summary.TotalWritten > 0 && cont.StateRepo != nil {
				target := domain.SkillsDirtyTarget{Project: true}
				if cont.ConfigRepo != nil && cont.ConfigRepo.GetProjectConfig().Mode == domain.ModeWorkspace {
					target = domain.SkillsDirtyTarget{Workspace: true}
				}
				if err := cont.StateRepo.MarkSkillsDirty(cmd.Context(), target); err != nil {
					return err
				}
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
	cmd.Flags().BoolVar(&useAI, "ai", false, i18n.Get("PatternsCompactFlagAI"))
	return cmd
}

func curatorProgressHooks(tracker *progress.Tracker) curator.ProgressHooks {
	return curator.ProgressHooks{
		OnStepStart:    tracker.StartStep,
		OnStepUpdate:   tracker.UpdateStep,
		OnStepComplete: tracker.CompleteStep,
	}
}
