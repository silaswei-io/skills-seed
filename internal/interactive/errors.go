package interactive

import "errors"

var (
	// ErrCanceled 表示用户主动取消交互流程。
	ErrCanceled = errors.New("interactive flow canceled")
)
