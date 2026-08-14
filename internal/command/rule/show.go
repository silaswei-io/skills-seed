package rule

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
	rulestore "github.com/silaswei-io/skills-seed/internal/infra/storage/rule"
	"github.com/spf13/cobra"
)

type summaryView struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Target           string   `json:"target"`
	AffectedProjects []string `json:"affected_projects,omitempty"`
	Paths            []string `json:"paths,omitempty"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
}

type detailView struct {
	summaryView
	Content   string `json:"content"`
	CreatedAt string `json:"created_at,omitempty"`
}

func showCmd(cont *container.Container) *cobra.Command {
	var format string
	var child string
	cmd := &cobra.Command{
		Use:     "show [rule-id]",
		Short:   i18n.Get("RuleShowShort"),
		Long:    i18n.Get("RuleShowLongDesc"),
		Example: i18n.Get("RuleShowExample"),
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil || cont.ConfigRepo == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if format != "table" && format != "json" {
				return fmt.Errorf("%s", i18n.GetWithParams("RuleShowUnsupportedFormat", map[string]interface{}{"Format": format}))
			}
			target, closeTarget, name, err := resolveTarget(cmd.Context(), cont, child)
			if err != nil {
				return err
			}
			if closeTarget != nil {
				defer closeTarget()
			}
			if target.RuleRepo == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			if len(args) == 1 {
				rule, err := target.RuleRepo.Get(args[0])
				if errors.Is(err, rulestore.ErrNotFound) {
					return fmt.Errorf("%s", i18n.GetWithParams("RuleShowNotFound", map[string]interface{}{"ID": args[0]}))
				}
				if err != nil {
					return err
				}
				view := newDetail(*rule, name)
				if format == "json" {
					return writeJSON(cmd.OutOrStdout(), view)
				}
				return writeDetails(cmd.OutOrStdout(), view)
			}
			rules, err := target.RuleRepo.List()
			if err != nil {
				return err
			}
			views := make([]summaryView, 0, len(rules))
			for _, rule := range rules {
				views = append(views, newSummary(rule, name))
			}
			if format == "json" {
				return writeJSON(cmd.OutOrStdout(), views)
			}
			return writeList(cmd.OutOrStdout(), views)
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", i18n.Get("RuleShowFlagFormat"))
	cmd.Flags().StringVar(&child, "child", "", i18n.Get("RuleShowFlagChild"))
	return cmd
}

func newSummary(rule domain.Rule, target string) summaryView {
	return summaryView{
		ID:               rule.ID,
		Name:             rule.Name,
		Target:           target,
		AffectedProjects: rule.AffectedProjects,
		Paths:            rule.Paths,
		UpdatedAt:        formatTime(rule.UpdatedAt),
	}
}

func newDetail(rule domain.Rule, target string) detailView {
	return detailView{summaryView: newSummary(rule, target), Content: rule.Content, CreatedAt: formatTime(rule.CreatedAt)}
}

func writeList(w io.Writer, rules []summaryView) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", i18n.Get("RuleShowHeaderID"), i18n.Get("RuleShowHeaderName"), i18n.Get("RuleShowHeaderTarget"), i18n.Get("RuleShowHeaderScope"), i18n.Get("RuleShowHeaderUpdatedAt")); err != nil {
		return err
	}
	for _, rule := range rules {
		scope := strings.Join(append(append([]string{}, rule.AffectedProjects...), rule.Paths...), ", ")
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", rule.ID, rule.Name, targetLabel(rule.Target), scope, rule.UpdatedAt); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writeDetails(w io.Writer, rule detailView) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fields := []struct{ label, value string }{
		{i18n.Get("RuleShowFieldID"), rule.ID},
		{i18n.Get("RuleShowFieldName"), rule.Name},
		{i18n.Get("RuleShowFieldTarget"), targetLabel(rule.Target)},
		{i18n.Get("RuleShowFieldProjects"), strings.Join(rule.AffectedProjects, ", ")},
		{i18n.Get("RuleShowFieldPaths"), strings.Join(rule.Paths, ", ")},
		{i18n.Get("RuleShowFieldCreatedAt"), rule.CreatedAt},
		{i18n.Get("RuleShowFieldUpdatedAt"), rule.UpdatedAt},
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
	_, err := fmt.Fprintf(w, "\n%s\n\n%s\n", i18n.Get("RuleShowContentHeading"), strings.TrimSpace(rule.Content))
	return err
}

func writeJSON(w io.Writer, value interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
