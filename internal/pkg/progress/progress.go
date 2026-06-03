// Package progress 提供轻量终端进度显示
//
// 约定：本包只负责渲染进度条，不内置业务文案。调用方必须传入已经翻译的标签，避免终端输出绕过国际化
package progress

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/mattn/go-runewidth"
)

var (
	frames              = []string{"|", "/", "-", "\\"}
	consoleMu           sync.Mutex
	progressActive      bool
	progressLineOpen    bool
	pendingConsoleLines []string
	terminalWidth       = currentTerminalWidth
)

// Tracker 以步骤为单位显示进度
// 交互式终端环境下刷新同一行；非交互式终端环境下降级为逐行输出，便于持续集成和日志采集
type Tracker struct {
	total   int
	done    int
	width   int
	enabled bool

	mu        sync.Mutex
	active    bool
	label     string
	frame     int
	startedAt time.Time
	elapsed   time.Duration
	stop      chan struct{}
	stopped   chan struct{}
}

type MultiTracker struct {
	mu            sync.Mutex
	label         string
	tasks         map[string]*multiTask
	order         []string
	width         int
	taskTotal     int
	enabled       bool
	lines         int
	physicalLines int
	stop          chan struct{}
	stopped       chan struct{}
}

type multiTask struct {
	name          string
	label         string
	frame         int
	doneSteps     int
	startedAt     time.Time
	elapsed       time.Duration
	done          bool
	active        bool
	animatePaused bool
}

// New 创建指定总步骤数的进度跟踪器
func New(total int) *Tracker {
	if total <= 0 {
		total = 1
	}
	return &Tracker{
		total:   total,
		width:   28,
		enabled: isTerminal(os.Stdout),
	}
}

func NewMulti(names []string) *MultiTracker {
	tracker := &MultiTracker{
		tasks:   make(map[string]*multiTask, len(names)),
		order:   append([]string(nil), names...),
		width:   28,
		enabled: isTerminal(os.Stdout),
	}
	for _, name := range names {
		tracker.tasks[name] = &multiTask{name: name}
	}
	return tracker
}

func (t *MultiTracker) SetLabel(label string) {
	t.mu.Lock()
	t.label = label
	t.mu.Unlock()
}

func (t *MultiTracker) SetTaskTotal(total int) {
	if total < 0 {
		total = 0
	}
	t.mu.Lock()
	t.taskTotal = total
	t.mu.Unlock()
}

func (t *MultiTracker) Start(name, label string) {
	t.mu.Lock()
	task := t.ensureTaskLocked(name)
	task.label = label
	task.frame = 0
	if task.startedAt.IsZero() || task.done {
		task.startedAt = time.Now()
		task.elapsed = 0
	}
	task.done = false
	task.active = true
	task.animatePaused = false
	shouldStartTicker := t.enabled && t.stop == nil
	if shouldStartTicker {
		t.stop = make(chan struct{})
		t.stopped = make(chan struct{})
	}
	lines := t.renderLinesLocked()
	t.mu.Unlock()

	if !t.enabled {
		printMultiLines(lines)
		return
	}
	if shouldStartTicker {
		go t.tick(t.stop, t.stopped)
	}
	t.Render()
}

func (t *MultiTracker) CompleteStep(name, label string) {
	t.mu.Lock()
	task := t.ensureTaskLocked(name)
	task.label = label
	if task.doneSteps < t.taskTotal || t.taskTotal <= 0 {
		task.doneSteps++
	}
	if task.startedAt.IsZero() {
		task.startedAt = time.Now()
	}
	task.elapsed = time.Since(task.startedAt).Truncate(time.Second)
	task.active = false
	task.animatePaused = true
	lines := t.renderLinesLocked()
	t.mu.Unlock()

	if !t.enabled {
		printMultiLines(lines)
		return
	}
	t.Render()
}

func (t *MultiTracker) Update(name, label string) {
	t.mu.Lock()
	task := t.ensureTaskLocked(name)
	task.label = label
	if !task.active && !task.done {
		task.startedAt = time.Now()
		task.active = true
		task.animatePaused = false
	}
	lines := t.renderLinesLocked()
	t.mu.Unlock()

	if !t.enabled {
		printMultiLines(lines)
		return
	}
	t.Render()
}

func (t *MultiTracker) Complete(name, label string) {
	t.mu.Lock()
	task := t.ensureTaskLocked(name)
	task.label = label
	if task.startedAt.IsZero() {
		task.startedAt = time.Now()
	}
	if t.taskTotal > 0 {
		task.doneSteps = t.taskTotal
	}
	task.elapsed = time.Since(task.startedAt).Truncate(time.Second)
	task.done = true
	task.active = false
	task.animatePaused = false
	allDone := t.allDoneLocked()
	stop := t.stop
	stopped := t.stopped
	if allDone {
		t.stop = nil
		t.stopped = nil
	}
	lines := t.renderLinesLocked()
	t.mu.Unlock()

	if !t.enabled {
		printMultiLines(lines)
		return
	}
	t.Render()
	if allDone && stop != nil {
		close(stop)
		<-stopped
		t.finish()
	}
}

func (t *MultiTracker) Fail(name, label string) {
	t.Complete(name, label)
}

func (t *MultiTracker) Render() {
	if !t.enabled {
		return
	}

	t.mu.Lock()
	lines := t.renderLinesLocked()
	width := terminalWidth()
	previousPhysicalLines := t.physicalLines
	t.lines = len(lines)
	t.physicalLines = renderedPhysicalLineCount(lines, width)
	t.mu.Unlock()

	consoleMu.Lock()
	defer consoleMu.Unlock()

	if previousPhysicalLines > 1 {
		fmt.Fprintf(os.Stdout, "\033[%dF", previousPhysicalLines-1)
	}
	if previousPhysicalLines > 0 {
		fmt.Fprint(os.Stdout, "\r\033[J")
	}
	for i, line := range lines {
		if i > 0 {
			fmt.Fprint(os.Stdout, "\n")
		}
		fmt.Fprintf(os.Stdout, "\r\033[2K%s", line)
	}
	progressActive = true
	progressLineOpen = true
}

func (t *MultiTracker) tick(stop <-chan struct{}, stopped chan<- struct{}) {
	defer close(stopped)

	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			t.mu.Lock()
			for _, task := range t.tasks {
				if task.active || task.animatePaused {
					task.frame++
				}
			}
			t.mu.Unlock()
			t.Render()
		}
	}
}

func (t *MultiTracker) finish() {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	if progressLineOpen {
		fmt.Fprintln(os.Stdout)
	}
	progressActive = false
	progressLineOpen = false
	flushPendingConsoleLinesLocked()
}

func (t *MultiTracker) ensureTaskLocked(name string) *multiTask {
	if task := t.tasks[name]; task != nil {
		return task
	}
	task := &multiTask{name: name}
	t.tasks[name] = task
	t.order = append(t.order, name)
	return task
}

func (t *MultiTracker) renderLinesLocked() []string {
	lines := make([]string, 0, len(t.order)+1)
	done := t.doneCountLocked()
	total := len(t.order)
	lines = append(lines, formatMultiSummaryLine(t.label, done, total, t.width))
	for _, name := range t.order {
		task := t.tasks[name]
		if task == nil {
			continue
		}
		lines = append(lines, formatMultiTaskLineWithTotal(task, t.width, t.taskTotal))
	}
	return lines
}

func (t *MultiTracker) doneCountLocked() int {
	done := 0
	for _, task := range t.tasks {
		if task.done {
			done++
		}
	}
	return done
}

func (t *MultiTracker) allDoneLocked() bool {
	if len(t.tasks) == 0 {
		return true
	}
	for _, task := range t.tasks {
		if !task.done {
			return false
		}
	}
	return true
}

func formatMultiSummaryLine(label string, done, total, width int) string {
	if total <= 0 {
		total = 1
	}
	if label == "" {
		label = "progress"
	}
	filled := int(float64(done) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
	return fmt.Sprintf("[%s] %d/%d %s", bar, done, total, label)
}

func formatMultiTaskLine(task *multiTask, width int) string {
	return formatMultiTaskLineWithTotal(task, width, 0)
}

func formatMultiTaskLineWithTotal(task *multiTask, width int, taskTotal int) string {
	filled := 0
	if task.done {
		filled = width
	} else if taskTotal > 0 {
		currentStep := task.doneSteps
		if task.active && currentStep < taskTotal {
			currentStep++
		}
		filled = int(float64(currentStep) / float64(taskTotal) * float64(width))
	} else if task.active {
		filled = width / 3
	}
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", width-filled)

	frame := " "
	elapsed := task.elapsed
	if task.active || task.animatePaused {
		frame = frames[task.frame%len(frames)]
		elapsed = time.Since(task.startedAt).Truncate(time.Second)
	}
	stepText := ""
	if taskTotal > 0 {
		currentStep := task.doneSteps
		if task.done {
			currentStep = taskTotal
		} else if task.active && currentStep < taskTotal {
			currentStep++
		}
		stepText = fmt.Sprintf(" %d/%d", currentStep, taskTotal)
	}
	line := fmt.Sprintf("[%s] %s %-12s%s %s", bar, frame, task.name, stepText, task.label)
	if elapsed > 0 {
		line += fmt.Sprintf(" (%s)", elapsed)
	}
	return line
}

func printMultiLines(lines []string) {
	for _, line := range lines {
		PrintConsoleLine(line)
	}
}

func renderedPhysicalLineCount(lines []string, width int) int {
	if width <= 0 {
		width = 80
	}
	count := 0
	for _, line := range lines {
		lineWidth := runewidth.StringWidth(line)
		physical := lineWidth / width
		if lineWidth%width != 0 || physical == 0 {
			physical++
		}
		count += physical
	}
	return count
}

func currentTerminalWidth() int {
	width, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || width <= 0 {
		return 80
	}
	return width
}

// RunStep 在执行回调期间显示该步骤的动态进度
func (t *Tracker) RunStep(label string, fn func() error) error {
	t.StartStep(label)
	if err := fn(); err != nil {
		t.FailStep(label)
		return err
	}
	t.CompleteStep(label)
	return nil
}

// StartStep 开始一个步骤
func (t *Tracker) StartStep(label string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled {
		PrintConsoleLine(label)
		return
	}

	t.active = true
	t.label = label
	t.frame = 0
	t.startedAt = time.Now()
	t.elapsed = 0
	t.stop = make(chan struct{})
	t.stopped = make(chan struct{})
	t.renderLocked(false)

	go t.tick(t.stop, t.stopped)
}

// CompleteStep 标记当前步骤完成
func (t *Tracker) CompleteStep(label string) {
	t.stopActiveTicker()

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.done < t.total {
		t.done++
	}
	if !t.enabled {
		return
	}
	t.label = label
	t.renderLocked(true)
}

func (t *Tracker) UpdateStep(label string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.label = label
	if !t.enabled || !t.active {
		return
	}
	t.renderLocked(false)
}

// FailStep 结束当前步骤并保留当前进度行，错误文案由调用方负责输出
func (t *Tracker) FailStep(label string) {
	t.stopActiveTicker()

	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled {
		return
	}
	t.label = label
	t.renderLocked(true)
}

func (t *Tracker) tick(stop <-chan struct{}, stopped chan<- struct{}) {
	defer close(stopped)

	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			t.mu.Lock()
			if t.active {
				t.frame++
				t.renderLocked(false)
			}
			t.mu.Unlock()
		}
	}
}

func (t *Tracker) stopActiveTicker() {
	t.mu.Lock()
	if !t.active {
		t.mu.Unlock()
		return
	}
	stop := t.stop
	stopped := t.stopped
	t.elapsed = time.Since(t.startedAt).Truncate(time.Second)
	t.active = false
	t.mu.Unlock()

	close(stop)
	<-stopped
}

func (t *Tracker) renderLocked(newline bool) {
	filled := int(float64(t.done) / float64(t.total) * float64(t.width))
	if filled > t.width {
		filled = t.width
	}
	bar := strings.Repeat("#", filled) + strings.Repeat("-", t.width-filled)

	step := t.done
	if t.active && step < t.total {
		step++
	}

	frame := " "
	elapsed := time.Duration(0)
	if t.active {
		frame = frames[t.frame%len(frames)]
		elapsed = time.Since(t.startedAt).Truncate(time.Second)
	} else {
		elapsed = t.elapsed
	}

	consoleMu.Lock()
	defer consoleMu.Unlock()

	fmt.Fprintf(os.Stdout, "\r\033[2K[%s] %d/%d %s %s", bar, step, t.total, frame, t.label)
	if elapsed > 0 {
		fmt.Fprintf(os.Stdout, " (%s)", elapsed)
	}
	if newline {
		fmt.Fprintln(os.Stdout)
		progressActive = false
		progressLineOpen = false
		flushPendingConsoleLinesLocked()
		return
	}
	progressActive = true
	progressLineOpen = true
}

// PrintConsoleLine 输出普通控制台消息；如果当前有正在刷新的进度行，会延迟到步骤结束后输出
func PrintConsoleLine(message string) {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	if progressActive {
		pendingConsoleLines = append(pendingConsoleLines, message)
		return
	}
	fmt.Fprintln(os.Stdout, message)
}

// PrintConsoleLineNow 立即输出普通控制台消息；如果当前有正在刷新的进度行，会先清除当前行
func PrintConsoleLineNow(message string) {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	if progressLineOpen {
		fmt.Fprint(os.Stdout, "\r\033[2K")
		progressLineOpen = false
	}
	fmt.Fprintln(os.Stdout, message)
}

// PrintConsoleLineAfterProgress 输出普通控制台消息；如果当前步骤仍在刷新，则延迟到步骤结束后输出
func PrintConsoleLineAfterProgress(message string) {
	consoleMu.Lock()
	defer consoleMu.Unlock()

	if progressActive {
		pendingConsoleLines = append(pendingConsoleLines, message)
		return
	}
	fmt.Fprintln(os.Stdout, message)
}

func flushPendingConsoleLinesLocked() {
	for _, message := range pendingConsoleLines {
		fmt.Fprintln(os.Stdout, message)
	}
	pendingConsoleLines = nil
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
