package loginutils

import (
	"net/url"
	"strings"
	"unicode"
)

// SafeRedirect permits only a path on the current origin, never a URL or a
// browser-normalized authority (including escaped slashes and backslashes).
func SafeRedirect(value string) string {
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return ""
	}
	decoded, err := url.PathUnescape(value)
	if err != nil || strings.HasPrefix(decoded, "//") || strings.Contains(decoded, "\\") || strings.IndexFunc(decoded, unicode.IsControl) >= 0 {
		return ""
	}
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.Opaque != "" {
		return ""
	}
	return value
}
