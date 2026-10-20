package core

import (
	"regexp"
	"strings"
)

var (
	uuidRegex  = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	wgKeyRegex = regexp.MustCompile(`[A-Za-z0-9+/]{42}[AEIMQUYcgkosw048]=`)

	// Secrets carried in the query string of subscription / share links, e.g.
	//   https://panel.example/sub?token=SECRET
	//   https://panel.example/api?key=SECRET&subid=42
	// The parameter name is preserved (keeps logs readable) while the value is
	// replaced. Without this, subscription tokens were written to the log in
	// clear text — the Privacy Shield used to only cover UUIDs, WireGuard keys
	// and scheme://user@host credentials.
	urlSecretRegex = regexp.MustCompile(
		`(?i)([?&](?:token|access_token|refresh_token|api_?key|key|auth|authorization|password|passwd|pwd|secret|license|signature|sig|sub|subid|sub_id|hash)=)([^&\s"'<>]+)`)
)

// RedactSensitiveInfo masks UUIDs, private keys, and sensitive tokens from log messages
func RedactSensitiveInfo(msg string) string {
	if msg == "" {
		return ""
	}

	// Mask UUIDs (keep first 4 and last 4 chars)
	msg = uuidRegex.ReplaceAllStringFunc(msg, func(uuid string) string {
		if len(uuid) >= 36 {
			return uuid[:4] + "-****-****-****-" + uuid[len(uuid)-4:]
		}
		return "********-****-****-****-************"
	})

	// Mask WireGuard / Private base64 keys
	msg = wgKeyRegex.ReplaceAllStringFunc(msg, func(key string) string {
		if len(key) >= 8 {
			return key[:4] + "****************" + key[len(key)-4:]
		}
		return "****************"
	})

	// Mask passwords in URLs (e.g. trojan://password@... or vless://uuid@...)
	if strings.Contains(msg, "://") && strings.Contains(msg, "@") {
		parts := strings.Split(msg, "://")
		if len(parts) == 2 {
			subParts := strings.SplitN(parts[1], "@", 2)
			if len(subParts) == 2 {
				msg = parts[0] + "://***REDACTED***@" + subParts[1]
			}
		}
	}

	// Mask secrets passed as query parameters (?token=..., &key=..., ...)
	msg = urlSecretRegex.ReplaceAllString(msg, "${1}***REDACTED***")

	return msg
}

// RedactURLSecrets masks only the sensitive query parameters of a URL. Unlike
// RedactSensitiveInfo it is meant to be applied unconditionally to values that
// are credentials by nature (subscription links), independent of the user's
// Privacy Shield setting.
func RedactURLSecrets(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	return urlSecretRegex.ReplaceAllString(rawURL, "${1}***REDACTED***")
}
