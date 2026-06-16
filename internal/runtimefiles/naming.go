package runtimefiles

import (
	"strings"
	"time"
)

// timestampLayout 是 runtime 记录文件名前缀的统一时间格式。
const timestampLayout = "20060102-150405.000000000"

// Name 生成 runtime 记录文件名前缀，统一以时间开头，方便按目录排序定位。
func Name(kind string, parts ...string) string {
	segments := []string{time.Now().Format(timestampLayout)}
	for _, value := range append([]string{kind}, parts...) {
		if safe := SafePart(value, "runtime"); safe != "" {
			segments = append(segments, safe)
		}
	}
	return strings.Join(segments, "-")
}

// TempPattern 生成 os.MkdirTemp 可用的 runtime 临时目录前缀。
func TempPattern(kind string, parts ...string) string {
	return Name(kind, parts...) + "-"
}

// SafePart 把业务片段收敛成文件名安全片段。
func SafePart(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	safe := strings.Trim(b.String(), "-_.")
	if safe == "" {
		return fallback
	}
	return safe
}
