package agent

import "regexp"

// HTTPStatusRetryableRegex matches standalone HTTP rate-limit/overload status codes
// (e.g. "status: 429", "HTTP 429", "HTTP/1.1 529"). This avoids false positives from
// normal output that merely contains the digits "429" or "529".
var HTTPStatusRetryableRegex = regexp.MustCompile(`(?i)(?:HTTP/1\.[01]\s+|status[:\s=]\s*|HTTP\s+|\b)(?:429|503|529)\b`)
