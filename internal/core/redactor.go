package core

import (
	"regexp"
	"strings"
)

var (
	uuidRegex  = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	wgKeyRegex = regexp.MustCompile(`[A-Za-z0-9+/]{42}[AEIMQUYcgkosw048]=`)
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

	return msg
}
