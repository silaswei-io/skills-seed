// Package colors 提供 ANSI 颜色常量和辅助函数
package colors

// ANSI 颜色常量
const (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m" // 也叫 Purple
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"
	Bold    = "\033[1m"
)

// 别名（兼容性）
const (
	Purple = Magenta
)

// Colorize 为文本添加颜色
func Colorize(color, text string) string {
	return color + text + Reset
}

// Redize 将文本标记为红色
func Redize(text string) string {
	return Colorize(Red, text)
}

// Greenize 将文本标记为绿色
func Greenize(text string) string {
	return Colorize(Green, text)
}

// Yellowize 将文本标记为黄色
func Yellowize(text string) string {
	return Colorize(Yellow, text)
}

// Blueize 将文本标记为蓝色
func Blueize(text string) string {
	return Colorize(Blue, text)
}

// Cyanize 将文本标记为青色
func Cyanize(text string) string {
	return Colorize(Cyan, text)
}

// Magentaize 将文本标记为洋红色
func Magentaize(text string) string {
	return Colorize(Magenta, text)
}

// Boldize 将文本加粗
func Boldize(text string) string {
	return Bold + text + Reset
}
