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
)

var (
	frames              = []string{"|", "/", "-", "\\"}
	consoleMu           sync.Mutex
	progressActive      bool
	progressLineOpen    bool
	pendingConsoleLines []string
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

// PrintConsoleLine 输出普通控制台消息；如果当前有正在刷新的进度行，会先换行
func PrintConsoleLine(message string) {
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
