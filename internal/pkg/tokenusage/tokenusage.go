package tokenusage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
)

// Usage 保存模型命令行返回的令牌与费用信息
type Usage struct {
	InputTokens              int64
	OutputTokens             int64
	TotalTokens              int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	ReasoningTokens          int64
	CostUSD                  float64
	HasTokens                bool
	HasCost                  bool
	Source                   string
	cacheReadIncludedInInput bool
}

var (
	mu    sync.Mutex
	total Usage
)

// Extract 解析结构化对象或逐行事件流，返回最可信的令牌消耗
// 优先使用命令行上报的总量，不做估算；同一份数据可能同时出现在响应和结束事件中，因此不会累加嵌套结果
func Extract(output string) Usage {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return Usage{}
	}

	if usage := extractJSON([]byte(trimmed), "json"); usage.Known() {
		return usage
	}

	var best Usage
	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	maxLineBytes := len(trimmed) + 1
	if maxLineBytes < 64*1024 {
		maxLineBytes = 64 * 1024
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if usage := extractJSON([]byte(line), "jsonl"); better(usage, best) {
			best = usage
		}
	}
	return best.Normalize()
}

// Record 将单次消耗计入进程累计值，并返回新的累计结果
func Record(usage Usage) Usage {
	usage = usage.Normalize()
	if !usage.Known() {
		return Snapshot()
	}

	mu.Lock()
	defer mu.Unlock()

	total.InputTokens += usage.InputTokens
	total.OutputTokens += usage.OutputTokens
	total.TotalTokens += usage.TotalTokens
	total.CacheCreationInputTokens += usage.CacheCreationInputTokens
	total.CacheReadInputTokens += usage.CacheReadInputTokens
	total.ReasoningTokens += usage.ReasoningTokens
	if usage.HasCost {
		total.CostUSD += usage.CostUSD
		total.HasCost = true
	}
	total.HasTokens = true
	total.Source = "cumulative"
	return total
}

// Snapshot 返回当前进程的累计消耗
func Snapshot() Usage {
	mu.Lock()
	defer mu.Unlock()
	return total
}

// Reset 清空进程累计值，便于测试隔离状态
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	total = Usage{}
}

// Known 判断是否包含可用的消耗信息
func (u Usage) Known() bool {
	return u.HasTokens || u.HasCost
}

// UncachedTokens 返回本次未从提示词缓存读取的输入、输出和缓存写入量。
func (u Usage) UncachedTokens() int64 {
	return u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens
}

// Normalize 补齐令牌总量和已知状态
func (u Usage) Normalize() Usage {
	if !u.HasTokens {
		u.HasTokens = u.InputTokens > 0 ||
			u.OutputTokens > 0 ||
			u.TotalTokens > 0 ||
			u.CacheCreationInputTokens > 0 ||
			u.CacheReadInputTokens > 0 ||
			u.ReasoningTokens > 0
	}
	if u.TotalTokens == 0 {
		u.TotalTokens = u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens
		if !u.cacheReadIncludedInInput {
			u.TotalTokens += u.CacheReadInputTokens
		}
	}
	return u
}

// Fields 将消耗信息转换为日志字段
func Fields(usage Usage, prefix string) []any {
	usage = usage.Normalize()
	fields := []any{prefix + "token_usage_available", usage.Known()}
	if !usage.Known() {
		return fields
	}

	fields = append(fields,
		prefix+"input_tokens", usage.InputTokens,
		prefix+"output_tokens", usage.OutputTokens,
		prefix+"total_tokens", usage.TotalTokens,
		prefix+"cache_creation_input_tokens", usage.CacheCreationInputTokens,
		prefix+"cache_read_input_tokens", usage.CacheReadInputTokens,
		prefix+"reasoning_tokens", usage.ReasoningTokens,
	)
	if usage.HasCost {
		fields = append(fields, prefix+"cost_usd", usage.CostUSD)
	}
	if usage.Source != "" {
		fields = append(fields, prefix+"token_usage_source", usage.Source)
	}
	return fields
}

// FormatCount 使用紧凑单位格式化数量
func FormatCount(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	switch {
	case value < 1_000:
		return fmt.Sprintf("%s%d", sign, value)
	case value < 1_000_000:
		return sign + trimUnit(float64(value)/1_000) + "k"
	case value < 1_000_000_000:
		return sign + trimUnit(float64(value)/1_000_000) + "m"
	default:
		return sign + trimUnit(float64(value)/1_000_000_000) + "b"
	}
}

// FormatCostUSD 格式化美元费用
func FormatCostUSD(value float64) string {
	if value == 0 {
		return "$0"
	}
	if math.Abs(value) < 0.01 {
		return fmt.Sprintf("$%.4f", value)
	}
	return fmt.Sprintf("$%.2f", value)
}

func extractJSON(data []byte, source string) Usage {
	var value interface{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return Usage{}
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return Usage{}
	}
	usage := extractValue(value, source)
	return usage.Normalize()
}

func trimUnit(value float64) string {
	formatted := fmt.Sprintf("%.1f", value)
	return strings.TrimSuffix(formatted, ".0")
}

func extractValue(value interface{}, source string) Usage {
	switch typed := value.(type) {
	case map[string]interface{}:
		self := usageFromMap(typed, source)
		best := self
		for key, child := range typed {
			childUsage := extractValue(child, source+"."+key)
			if better(childUsage, best) {
				best = childUsage
			}
		}
		if self.HasCost && !best.HasCost {
			best.CostUSD = self.CostUSD
			best.HasCost = true
		}
		return best.Normalize()
	case []interface{}:
		var best Usage
		for _, child := range typed {
			childUsage := extractValue(child, source)
			if better(childUsage, best) {
				best = childUsage
			}
		}
		return best.Normalize()
	default:
		return Usage{}
	}
}

func usageFromMap(data map[string]interface{}, source string) Usage {
	usage := Usage{Source: source}

	usage.InputTokens = firstInt(data, "input_tokens", "prompt_tokens", "input_token_count", "prompt_token_count")
	usage.OutputTokens = firstInt(data, "output_tokens", "completion_tokens", "output_token_count", "completion_token_count")
	usage.TotalTokens = firstInt(data, "total_tokens", "total_token_count")
	usage.CacheCreationInputTokens = firstInt(data,
		"cache_creation_input_tokens",
		"cache_creation_input_token_count",
		"cache_write_input_tokens",
		"cache_write_input_token_count",
		"cache_write_tokens",
		"cache_write_token_count",
	)
	usage.CacheReadInputTokens = firstInt(data, "cache_read_input_tokens", "cache_read_input_token_count")
	if usage.CacheReadInputTokens == 0 {
		usage.CacheReadInputTokens = firstInt(data, "cached_input_tokens", "cached_tokens", "cached_token_count")
		usage.cacheReadIncludedInInput = usage.CacheReadInputTokens > 0
	}
	usage.ReasoningTokens = firstInt(data, "reasoning_tokens", "reasoning_output_tokens", "reasoning_token_count", "reasoning_output_token_count")

	if details, ok := data["input_tokens_details"].(map[string]interface{}); ok {
		cached := firstInt(details, "cached_tokens", "cached_token_count", "cache_read_input_tokens", "cache_read_input_token_count")
		usage.CacheReadInputTokens += cached
		if cached > 0 {
			usage.cacheReadIncludedInInput = true
		}
	}
	if details, ok := data["output_tokens_details"].(map[string]interface{}); ok {
		usage.ReasoningTokens += firstInt(details, "reasoning_tokens", "reasoning_output_tokens", "reasoning_token_count", "reasoning_output_token_count")
	}

	if cost, ok := firstFloat(data, "cost_usd", "total_cost_usd", "total_cost", "cost"); ok {
		usage.CostUSD = cost
		usage.HasCost = true
	}

	return usage.Normalize()
}

func better(candidate, current Usage) bool {
	candidate = candidate.Normalize()
	current = current.Normalize()
	if !candidate.Known() {
		return false
	}
	if !current.Known() {
		return true
	}
	if candidate.TotalTokens != current.TotalTokens {
		return candidate.TotalTokens > current.TotalTokens
	}
	if candidate.InputTokens+candidate.OutputTokens != current.InputTokens+current.OutputTokens {
		return candidate.InputTokens+candidate.OutputTokens > current.InputTokens+current.OutputTokens
	}
	return candidate.HasCost && !current.HasCost
}

func firstInt(data map[string]interface{}, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := numberAsInt(data[key]); ok {
			return value
		}
	}
	return 0
}

func firstFloat(data map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := numberAsFloat(data[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func numberAsInt(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return n, true
		}
		if f, err := typed.Float64(); err == nil {
			return int64(math.Round(f)), true
		}
	case float64:
		return int64(math.Round(typed)), true
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	}
	return 0, false
}

func numberAsFloat(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		f, err := typed.Float64()
		return f, err == nil
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	}
	return 0, false
}
