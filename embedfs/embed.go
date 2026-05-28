package embedfs

import "embed"

// FS 保存嵌入的模板文件
//
//go:embed templates
var FS embed.FS
