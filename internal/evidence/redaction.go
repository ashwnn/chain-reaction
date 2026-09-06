package evidence

import (
	"regexp"
	"strings"
)

const redactedValue = "[redacted]"

var (
	authorizationValuePattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:bearer\s+)?[^\s,;]+`)
	bearerTokenPattern        = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`)
	jwtPattern                = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
)

// RedactValue returns a deep copy of JSON-like data with sensitive fields and
// credential-bearing text replaced before it reaches an artifact or planner.
func RedactValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		return RedactString(v)
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, nested := range v {
			if IsSensitiveKey(key) {
				out[key] = redactedValue
				continue
			}
			out[key] = RedactValue(nested)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(v))
		for key, nested := range v {
			if IsSensitiveKey(key) {
				out[key] = redactedValue
				continue
			}
			out[key] = RedactString(nested)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, nested := range v {
			out[i] = RedactValue(nested)
		}
		return out
	case []string:
		out := make([]string, len(v))
		for i, nested := range v {
			out[i] = RedactString(nested)
		}
		return out
	default:
		return value
	}
}

func RedactString(value string) string {
	value = authorizationValuePattern.ReplaceAllString(value, "${1}"+redactedValue)
	value = bearerTokenPattern.ReplaceAllString(value, "${1}"+redactedValue)
	return jwtPattern.ReplaceAllString(value, redactedValue)
}

func IsSensitiveKey(key string) bool {
	key = strings.ToLower(key)
	for _, token := range []string{"token", "secret", "password", "credential", "authorization", "api_key", "apikey", "private_key"} {
		if strings.Contains(key, token) {
			return true
		}
	}
	return false
}
