package agent

import "regexp"

// HTTPStatusRetryableRegex matches HTTP status codes only when nearby text makes
// them look like transport/API failures, not normal generated content numbers.
var HTTPStatusRetryableRegex = regexp.MustCompile(`(?i)(?:HTTP(?:/1\.[01])?\s+|status[:\s=]\s*|API Error:\s*)(?:429|503|529)\b`)
