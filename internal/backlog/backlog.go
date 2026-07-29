// Package backlog records intentionally removed product areas.
//
// 这里不注册命令、不挂生产流程，只保留后续重新设计时需要重新评估的能力清单。
package backlog

// Item 描述一个暂不进入核心实现的能力。
type Item struct {
	ID        string
	ReasonKey string
}

// Items 是当前从核心路径移出的能力清单。
var Items = []Item{
	{ID: "check", ReasonKey: "BacklogCheckReason"},
	{ID: "learn-history", ReasonKey: "BacklogLearnHistoryReason"},
	{ID: "autofix", ReasonKey: "BacklogAutofixReason"},
	{ID: "review-metrics", ReasonKey: "BacklogReviewMetricsReason"},
}
