// Package metadata provides structured parsing and validation for account metadata JSON.
// Account metadata supports flexible configuration like proxy, region, tags, notes, etc.
package metadata

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// AccountMetadata defines the standard structure for account metadata JSON.
// This struct provides type-safe access to metadata fields stored as JSON in the database.
type AccountMetadata struct {
	ProxyURL      string   `json:"proxy_url,omitempty"`       // Proxy URL (e.g., socks5://user:pass@host:port)
	ProxyEnabled  bool     `json:"proxy_enabled,omitempty"`   // Whether proxy is enabled
	Region        string   `json:"region,omitempty"`          // Geographic region (e.g., us-east, eu-west)
	Tags          []string `json:"tags,omitempty"`            // Tags for filtering (e.g., ["production", "team-a"])
	Notes         string   `json:"notes,omitempty"`           // Admin notes (max 500 chars)
	CustomBaseURL string   `json:"custom_base_url,omitempty"` // Custom API base URL for enterprise deployments
}

// Parse parses JSON string into AccountMetadata struct.
// Returns error if JSON is invalid or empty string returns empty metadata.
func Parse(jsonStr string) (*AccountMetadata, error) {
	if jsonStr == "" {
		return &AccountMetadata{}, nil
	}

	var meta AccountMetadata
	if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	return &meta, nil
}

// String serializes AccountMetadata to JSON string.
// Returns empty string if metadata is empty (all zero values).
func (m *AccountMetadata) String() string {
	// Check if metadata is empty (all zero values)
	if m.IsEmpty() {
		return ""
	}

	data, err := json.Marshal(m)
	if err != nil {
		return ""
	}

	return string(data)
}

// IsEmpty checks if metadata has any non-zero values.
func (m *AccountMetadata) IsEmpty() bool {
	return m.ProxyURL == "" &&
		!m.ProxyEnabled &&
		m.Region == "" &&
		len(m.Tags) == 0 &&
		m.Notes == "" &&
		m.CustomBaseURL == ""
}

// Validate validates metadata fields and returns error if invalid.
// Validation rules:
// - proxy_url: must be valid socks5:// or http(s):// URL if provided
// - custom_base_url: must be valid HTTPS URL if provided
// - tags: max 10 tags, each tag max 50 characters
// - notes: max 500 characters
func (m *AccountMetadata) Validate() error {
	// Validate proxy_url format
	if m.ProxyURL != "" {
		if err := validateProxyURL(m.ProxyURL); err != nil {
			return fmt.Errorf("invalid proxy_url: %w", err)
		}
	}

	// Validate custom_base_url format (must be HTTPS)
	if m.CustomBaseURL != "" {
		parsedURL, err := url.Parse(m.CustomBaseURL)
		if err != nil {
			return fmt.Errorf("invalid custom_base_url: %w", err)
		}
		if parsedURL.Scheme != "https" {
			return fmt.Errorf("custom_base_url must use HTTPS scheme, got: %s", parsedURL.Scheme)
		}
	}

	// Validate tags count and length
	if len(m.Tags) > 10 {
		return fmt.Errorf("too many tags: max 10 allowed, got %d", len(m.Tags))
	}
	for i, tag := range m.Tags {
		if len(tag) > 50 {
			return fmt.Errorf("tag[%d] too long: max 50 characters, got %d", i, len(tag))
		}
		if tag == "" {
			return fmt.Errorf("tag[%d] is empty", i)
		}
	}

	// Validate notes length
	if len(m.Notes) > 500 {
		return fmt.Errorf("notes too long: max 500 characters, got %d", len(m.Notes))
	}

	return nil
}

// MaskSensitive returns a copy of metadata with sensitive fields masked.
// Specifically, masks the password in proxy_url (e.g., socks5://user:***@host:port).
// This should be called before returning metadata to API clients.
func (m *AccountMetadata) MaskSensitive() *AccountMetadata {
	masked := *m // Copy struct

	// Mask proxy_url password
	if masked.ProxyURL != "" {
		masked.ProxyURL = maskProxyPassword(masked.ProxyURL)
	}

	return &masked
}

// validateProxyURL validates proxy URL format.
// Supports socks5://, socks5h://, http://, https:// schemes.
func validateProxyURL(proxyURL string) error {
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return err
	}

	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "socks5", "socks5h", "http", "https":
		return nil
	default:
		return fmt.Errorf("unsupported proxy scheme: %s (supported: socks5, socks5h, http, https)", scheme)
	}
}

// maskProxyPassword masks the password in proxy URL.
// Example: socks5://user:password@host:1080 -> socks5://user:***@host:1080
func maskProxyPassword(proxyURL string) string {
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return proxyURL // Return original if parsing fails
	}

	// Check if URL has user info
	if parsed.User == nil {
		return proxyURL // No user info, return as-is
	}

	username := parsed.User.Username()
	password, hasPassword := parsed.User.Password()
	if !hasPassword || password == "" {
		return proxyURL // No password, return as-is
	}

	// Manually construct URL to avoid URL encoding of "***"
	// Format: scheme://username:***@host:port/path
	scheme := parsed.Scheme
	host := parsed.Host
	path := parsed.Path
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		path += "#" + parsed.Fragment
	}

	return fmt.Sprintf("%s://%s:***@%s%s", scheme, username, host, path)
}
