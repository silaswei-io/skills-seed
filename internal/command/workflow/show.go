package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	workflowstore "github.com/silaswei-io/skills-seed/internal/infra/storage/workflow"
	workflowoutput "github.com/silaswei-io/skills-seed/internal/service/workflow/output"
	"github.com/spf13/cobra"
)

type workflowSummaryView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Target       string `json:"target"`
	Summary      string `json:"summary,omitempty"`
	ContextCount int    `json:"context_count"`
	ScriptCount  int    `json:"script_count"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type workflowContextView struct {
	Content   string `json:"content"`
	CreatedAt string `json:"created_at,omitempty"`
}

type workflowScriptView struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode"`
}

type workflowDetailView struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Target    string                `json:"target"`
	Summary   string                `json:"summary,omitempty"`
	Content   string                `json:"content"`
	Contexts  []workflowContextView `json:"contexts"`
	Scripts   []workflowScriptView  `json:"scripts"`
	CreatedAt string                `json:"created_at,omitempty"`
	UpdatedAt string                `json:"updated_at,omitempty"`
}

func showCmd(cont *container.Container) *cobra.Command {
	var format string
	var child string

	cmd := &cobra.Command{
		Use:     "show [workflow-id]",
		Short:   i18n.Get("WorkflowShowShort"),
		Long:    i18n.Get("WorkflowShowLongDesc"),
		Example: i18n.Get("WorkflowShowExample"),
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil || cont.ConfigRepo == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if format != "table" && format != "json" {
				return fmt.Errorf("%s", i18n.GetWithParams("WorkflowShowUnsupportedFormat", map[string]interface{}{"Format": format}))
			}
			targetCont, closeTarget, targetName, err := resolveWorkflowTarget(cmd.Context(), cont, child)
			if err != nil {
				return err
			}
			if closeTarget != nil {
				defer closeTarget()
			}
			if targetCont == nil || targetCont.WorkflowRepo == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}

			locale := targetCont.ConfigRepo.GetSkillsLocale()
			if len(args) == 1 {
				workflow, err := targetCont.WorkflowRepo.Get(args[0])
				if errors.Is(err, workflowstore.ErrNotFound) {
					return fmt.Errorf("%s", i18n.GetWithParams("WorkflowShowNotFound", map[string]interface{}{"ID": args[0]}))
				}
				if err != nil {
					return err
				}
				detail := newWorkflowDetailView(*workflow, targetName, locale)
				if format == "json" {
					return writeWorkflowJSON(cmd.OutOrStdout(), detail)
				}
				return writeWorkflowDetails(cmd.OutOrStdout(), detail)
			}

			workflows, err := targetCont.WorkflowRepo.List()
			if err != nil {
				return err
			}
			summaries := newWorkflowSummaryViews(workflows, targetName, locale)
			if format == "json" {
				return writeWorkflowJSON(cmd.OutOrStdout(), summaries)
			}
			return writeWorkflowList(cmd.OutOrStdout(), summaries)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", i18n.Get("WorkflowShowFlagFormat"))
	cmd.Flags().StringVar(&child, "child", "", i18n.Get("WorkflowShowFlagChild"))
	return cmd
}

func newWorkflowSummaryViews(workflows []domain.Workflow, target, locale string) []workflowSummaryView {
	views := make([]workflowSummaryView, 0, len(workflows))
	for _, workflow := range workflows {
		views = append(views, newWorkflowSummaryView(workflow, target, locale))
	}
	return views
}

func newWorkflowSummaryView(workflow domain.Workflow, target, locale string) workflowSummaryView {
	return workflowSummaryView{
		ID:           workflow.ID,
		Name:         workflow.Name,
		Target:       target,
		Summary:      workflowoutput.Summary(workflow, locale),
		ContextCount: len(workflow.Contexts),
		ScriptCount:  len(workflow.Scripts),
		UpdatedAt:    formatWorkflowTime(workflow.UpdatedAt),
	}
}

func newWorkflowDetailView(workflow domain.Workflow, target, locale string) workflowDetailView {
	contexts := make([]workflowContextView, 0, len(workflow.Contexts))
	for _, item := range workflow.Contexts {
		contexts = append(contexts, workflowContextView{
			Content:   item.Content,
			CreatedAt: formatWorkflowTime(item.CreatedAt),
		})
	}
	scripts := make([]workflowScriptView, 0, len(workflow.Scripts))
	for _, script := range workflow.Scripts {
		scripts = append(scripts, workflowScriptView{
			Path:   script.Path,
			SHA256: script.SHA256,
			Mode:   script.Mode,
		})
	}
	return workflowDetailView{
		ID:        workflow.ID,
		Name:      workflow.Name,
		Target:    target,
		Summary:   workflowoutput.Summary(workflow, locale),
		Content:   workflow.Content,
		Contexts:  contexts,
		Scripts:   scripts,
		CreatedAt: formatWorkflowTime(workflow.CreatedAt),
		UpdatedAt: formatWorkflowTime(workflow.UpdatedAt),
	}
}

func writeWorkflowList(w io.Writer, workflows []workflowSummaryView) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintf(
		tw,
		"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		i18n.Get("WorkflowShowHeaderID"),
		i18n.Get("WorkflowShowHeaderName"),
		i18n.Get("WorkflowShowHeaderTarget"),
		i18n.Get("WorkflowShowHeaderSummary"),
		i18n.Get("WorkflowShowHeaderContexts"),
		i18n.Get("WorkflowShowHeaderScripts"),
		i18n.Get("WorkflowShowHeaderUpdatedAt"),
	); err != nil {
		return err
	}
	for _, workflow := range workflows {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%d\t%d\t%s\n",
			workflow.ID,
			workflow.Name,
			workflow.Target,
			workflow.Summary,
			workflow.ContextCount,
			workflow.ScriptCount,
			workflow.UpdatedAt,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeWorkflowDetails(w io.Writer, workflow workflowDetailView) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fields := []struct {
		label string
		value string
	}{
		{i18n.Get("WorkflowShowFieldID"), workflow.ID},
		{i18n.Get("WorkflowShowFieldName"), workflow.Name},
		{i18n.Get("WorkflowShowFieldTarget"), workflow.Target},
		{i18n.Get("WorkflowShowFieldSummary"), workflow.Summary},
		{i18n.Get("WorkflowShowFieldContexts"), fmt.Sprintf("%d", len(workflow.Contexts))},
		{i18n.Get("WorkflowShowFieldScripts"), fmt.Sprintf("%d", len(workflow.Scripts))},
		{i18n.Get("WorkflowShowFieldCreatedAt"), workflow.CreatedAt},
		{i18n.Get("WorkflowShowFieldUpdatedAt"), workflow.UpdatedAt},
	}
	for _, field := range fields {
		if field.value == "" {
			continue
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", field.label, field.value); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if strings.TrimSpace(workflow.Content) == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "\n%s\n\n%s\n", i18n.Get("WorkflowShowContentHeading"), strings.TrimSpace(workflow.Content))
	return err
}

func writeWorkflowJSON(w io.Writer, value interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func formatWorkflowTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
