package audit

import "context"

// Log is deliberately generic fixture noise and must not define a business domain by keyword alone.
func Log(context.Context, string) {}
