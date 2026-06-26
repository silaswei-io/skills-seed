package runtimefiles

import (
	"strings"
	"time"
)

// timestampLayout 是 runtime 记录文件名前缀的日期时间格式。
const timestampLayout = "20060102-150405"

// MaxSafePartLength 限制单个业务片段长度，避免 runtime 文件名过长。
const MaxSafePartLength = 64

// Name 生成 runtime 记录文件名前缀，统一以时间开头，方便按目录排序定位。
func Name(kind string, parts ...string) string {
	return NameWithID(NewID(), kind, parts...)
}

// NewID 生成一次 runtime 任务可复用的短 ID。
func NewID() string {
	return time.Now().Format(timestampLayout)
}

// NameWithID 使用指定 runtime ID 生成记录文件名前缀。
func NameWithID(id, kind string, parts ...string) string {
	if strings.TrimSpace(id) == "" {
		id = NewID()
	}
	segments := []string{SafePart(id, NewID())}
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
	if count := len([]rune(safe)); count > MaxSafePartLength {
		runes := []rune(safe)
		safe = strings.Trim(string(runes[:MaxSafePartLength]), "-_.")
		if safe == "" {
			return fallback
		}
	}
	return safe
}
