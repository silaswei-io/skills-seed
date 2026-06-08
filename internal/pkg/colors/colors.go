// Package colors 提供 ANSI 颜色常量和辅助函数
package colors

// ANSI 颜色常量
const (
	// Reset 清除前面设置的颜色或样式。
	Reset = "\033[0m"
	// Red 前景色红色，用于错误或高风险信息。
	Red = "\033[31m"
	// Green 前景色绿色，用于成功信息。
	Green = "\033[32m"
	// Yellow 前景色黄色，用于警告信息。
	Yellow = "\033[33m"
	// Blue 前景色蓝色，用于普通强调信息。
	Blue = "\033[34m"
	// Magenta 前景色洋红色。
	Magenta = "\033[35m" // 也叫 Purple
	// Cyan 前景色青色。
	Cyan = "\033[36m"
	// White 前景色白色。
	White = "\033[37m"
	// Gray 前景色灰色。
	Gray = "\033[90m"
	// Bold 开启加粗样式。
	Bold = "\033[1m"
)

// 别名（兼容性）
const (
	// Purple 是 Magenta 的兼容别名。
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
