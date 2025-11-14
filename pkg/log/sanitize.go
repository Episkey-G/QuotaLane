package log

import (
	"strings"
)

// SanitizeField checks if the key contains sensitive keywords and sanitizes the value
func SanitizeField(key, value string) string {
	if value == "" {
		return value
	}

	// Convert key to lowercase for case-insensitive matching
	lowerKey := strings.ToLower(key)

	// Check if key contains sensitive keywords
	sensitiveKeywords := []string{
		"password", "passwd", "pwd",
		"api_key", "apikey", "api-key",
		"token", "access_token", "refresh_token",
		"secret", "auth", "authorization",
		"credential", "private_key", "privatekey",
	}

	isSensitive := false
	for _, keyword := range sensitiveKeywords {
		if strings.Contains(lowerKey, keyword) {
			isSensitive = true
			break
		}
	}

	// Special handling for email
	if strings.Contains(lowerKey, "email") || strings.Contains(lowerKey, "mail") {
		return sanitizeEmail(value)
	}

	// Sanitize sensitive fields
	if isSensitive {
		return sanitizeToken(value)
	}

	return value
}

// sanitizeToken masks token/password values showing only first 4 and last 4 characters
func sanitizeToken(value string) string {
	if len(value) <= 8 {
		// For short strings, mask everything except first and last char
		if len(value) <= 2 {
			return strings.Repeat("*", len(value))
		}
		return string(value[0]) + strings.Repeat("*", len(value)-2) + string(value[len(value)-1])
	}

	// For longer strings, show first 4 and last 4
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}

// sanitizeEmail masks email showing first 3 characters + @domain
func sanitizeEmail(value string) string {
	parts := strings.Split(value, "@")
	if len(parts) != 2 {
		// Invalid email format, mask everything
		return strings.Repeat("*", len(value))
	}

	localPart := parts[0]
	domain := parts[1]

	if len(localPart) <= 3 {
		// Short local part, show first char only
		if len(localPart) == 0 {
			return "@" + domain
		}
		return string(localPart[0]) + strings.Repeat("*", len(localPart)-1) + "@" + domain
	}

	// Show first 3 characters + *** + @domain
	return localPart[:3] + "***@" + domain
}
