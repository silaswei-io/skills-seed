package view

import (
	"context"
	"fmt"

	"github.com/silaswei-io/skills-seed/internal/container"
	"github.com/silaswei-io/skills-seed/internal/domain"
	"github.com/silaswei-io/skills-seed/internal/i18n"
	"github.com/silaswei-io/skills-seed/internal/pkg/logger"
	"github.com/spf13/cobra"
)

// Cmd 返回 view 命令
func Cmd(cont *container.Container) *cobra.Command {
	viewCmd := &cobra.Command{
		Use:     "view",
		Short:   i18n.Get("ViewShort"),
		Long:    i18n.Get("ViewLongDesc"),
		Example: i18n.Get("ViewExample"),
	}

	viewCmd.AddCommand(patternsCmd(cont))

	return viewCmd
}

func patternsCmd(cont *container.Container) *cobra.Command {
	var categoryFilter string

	cmd := &cobra.Command{
		Use:     "patterns",
		Short:   i18n.Get("ViewPatternsShort"),
		Long:    i18n.Get("ViewPatternsLong"),
		Example: i18n.Get("ViewPatternsExample"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cont == nil {
				return fmt.Errorf("%s", i18n.Get("ErrNotInitialized"))
			}
			return listPatterns(cont, categoryFilter)
		},
	}

	cmd.Flags().StringVarP(&categoryFilter, "category", "c", "", i18n.Get("ViewFlagCategory"))

	return cmd
}

func listPatterns(cont *container.Container, categoryFilter string) error {
	ctx := context.Background()

	patterns, err := cont.GetPatternRepository().GetAll(ctx)
	if err != nil {
		return err
	}

	if len(patterns) == 0 {
		logger.Info(i18n.Get("ViewNoPatterns"))
		return nil
	}

	// 按分类组织模式
	categoryMap := make(map[string][]domain.Pattern)
	for _, p := range patterns {
		category := string(p.Category)
		// 如果有过滤条件,只显示匹配的分类
		if categoryFilter != "" && category != categoryFilter {
			continue
		}
		categoryMap[category] = append(categoryMap[category], p)
	}

	// 分类显示顺序（按优先级）
	categoryOrder := []string{
		"business",    // 业务逻辑 - 最高优先级
		"database",    // 数据库
		"api",         // API设计
		"naming",      // 命名
		"error",       // 错误处理
		"structure",   // 代码结构
		"concurrency", // 并发
		"utils",       // 工具方法
		"middleware",  // 中间件
		"config",      // 配置管理
		"testing",     // 测试
	}

	// 如果没有匹配的模式
	if len(categoryMap) == 0 {
		if categoryFilter != "" {
			logger.Info(i18n.GetWithParams("ViewNoPatternsInCategory", map[string]interface{}{"Category": categoryFilter}))
		} else {
			logger.Info(i18n.Get("ViewNoPatterns"))
		}
		return nil
	}

	// 显示总体统计
	logger.Info(i18n.GetWithParams("ViewPatternsTotal", map[string]interface{}{"Count": len(patterns)}))
	logger.Info("")

	// 分类名称映射到 i18n 键
	categoryI18nKeys := map[string]string{
		"business":    "ViewCategoryBusiness",
		"database":    "ViewCategoryDatabase",
		"api":         "ViewCategoryAPI",
		"naming":      "ViewCategoryNaming",
		"error":       "ViewCategoryError",
		"structure":   "ViewCategoryStructure",
		"concurrency": "ViewCategoryConcurrency",
		"testing":     "ViewCategoryTesting",
		"utils":       "ViewCategoryUtils",
		"middleware":  "ViewCategoryMiddleware",
		"config":      "ViewCategoryConfig",
	}

	// 按分类显示
	for _, category := range categoryOrder {
		patternsInCategory, exists := categoryMap[category]
		if !exists || len(patternsInCategory) == 0 {
			continue
		}

		// 获取分类名称
		i18nKey, ok := categoryI18nKeys[category]
		if !ok {
			i18nKey = "ViewCategoryUnknown"
		}
		categoryName := i18n.Get(i18nKey)

		// 计算平均置信度
		var totalConfidence float64
		for _, p := range patternsInCategory {
			totalConfidence += p.Confidence
		}
		avgConfidence := totalConfidence / float64(len(patternsInCategory))

		// 显示分类标题
		logger.Info(i18n.GetWithParams("ViewCategoryHeader", map[string]interface{}{
			"Category":   categoryName,
			"Count":      len(patternsInCategory),
			"Confidence": avgConfidence * 100,
		}))
		logger.Info(i18n.Get("ViewSeparatorLine"))

		// 显示该分类下的模式
		for i, pattern := range patternsInCategory {
			logger.Info(i18n.GetWithParams("ViewPatternName", map[string]interface{}{
				"Index": i + 1,
				"Name":  pattern.Name,
			}))
			logger.Info(i18n.GetWithParams("ViewPatternFrequency", map[string]interface{}{
				"Confidence": pattern.Confidence * 100,
				"Frequency":  pattern.Frequency,
			}))

			desc := pattern.Description
			if len(desc) > 100 {
				desc = desc[:97] + "..."
			}
			logger.Info(i18n.GetWithParams("ViewPatternDesc", map[string]interface{}{"Description": desc}))
			logger.Info("")
		}
		logger.Info("")
	}

	// 显示统计摘要
	logger.Info(i18n.Get("ViewDoubleLine"))
	logger.Info(i18n.Get("ViewStatsSummary"))
	logger.Info(i18n.Get("ViewDoubleLine"))
	for _, category := range categoryOrder {
		if patternsInCategory, exists := categoryMap[category]; exists {
			i18nKey, ok := categoryI18nKeys[category]
			if !ok {
				i18nKey = "ViewCategoryUnknown"
			}
			categoryName := i18n.Get(i18nKey)
			logger.Info(i18n.GetWithParams("ViewStatsCategoryLine", map[string]interface{}{
				"Category": categoryName,
				"Count":    len(patternsInCategory),
			}))
		}
	}
	logger.Info("")

	return nil
}
