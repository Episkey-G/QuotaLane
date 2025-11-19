package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	t.Run("parse valid JSON", func(t *testing.T) {
		jsonStr := `{"proxy_url":"socks5://user:pass@proxy.example.com:1080","proxy_enabled":true,"region":"us-east","tags":["production","team-a"],"notes":"Test account"}`

		meta, err := Parse(jsonStr)

		assert.NoError(t, err)
		assert.Equal(t, "socks5://user:pass@proxy.example.com:1080", meta.ProxyURL)
		assert.True(t, meta.ProxyEnabled)
		assert.Equal(t, "us-east", meta.Region)
		assert.Equal(t, []string{"production", "team-a"}, meta.Tags)
		assert.Equal(t, "Test account", meta.Notes)
	})

	t.Run("parse empty string", func(t *testing.T) {
		meta, err := Parse("")

		assert.NoError(t, err)
		assert.NotNil(t, meta)
		assert.True(t, meta.IsEmpty())
	})

	t.Run("parse invalid JSON", func(t *testing.T) {
		meta, err := Parse("{invalid json")

		assert.Error(t, err)
		assert.Nil(t, meta)
		assert.Contains(t, err.Error(), "failed to parse metadata JSON")
	})
}

func TestString(t *testing.T) {
	t.Run("serialize non-empty metadata", func(t *testing.T) {
		meta := &AccountMetadata{
			ProxyURL:     "socks5://proxy.example.com:1080",
			ProxyEnabled: true,
			Region:       "us-east",
			Tags:         []string{"production"},
		}

		jsonStr := meta.String()

		assert.NotEmpty(t, jsonStr)
		assert.Contains(t, jsonStr, "socks5://proxy.example.com:1080")
		assert.Contains(t, jsonStr, "us-east")
		assert.Contains(t, jsonStr, "production")
	})

	t.Run("serialize empty metadata", func(t *testing.T) {
		meta := &AccountMetadata{}

		jsonStr := meta.String()

		assert.Empty(t, jsonStr)
	})
}

func TestValidate(t *testing.T) {
	t.Run("valid metadata", func(t *testing.T) {
		meta := &AccountMetadata{
			ProxyURL:      "socks5://user:pass@proxy.example.com:1080",
			ProxyEnabled:  true,
			Region:        "us-east",
			Tags:          []string{"production", "team-a"},
			Notes:         "Test account",
			CustomBaseURL: "https://api.custom.com",
		}

		err := meta.Validate()

		assert.NoError(t, err)
	})

	t.Run("invalid proxy URL scheme", func(t *testing.T) {
		meta := &AccountMetadata{
			ProxyURL: "ftp://proxy.example.com:1080",
		}

		err := meta.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported proxy scheme: ftp")
	})

	t.Run("valid socks5 proxy", func(t *testing.T) {
		meta := &AccountMetadata{
			ProxyURL: "socks5://user:pass@proxy.example.com:1080",
		}

		err := meta.Validate()

		assert.NoError(t, err)
	})

	t.Run("valid http proxy", func(t *testing.T) {
		meta := &AccountMetadata{
			ProxyURL: "http://proxy.example.com:8080",
		}

		err := meta.Validate()

		assert.NoError(t, err)
	})

	t.Run("invalid custom_base_url (non-HTTPS)", func(t *testing.T) {
		meta := &AccountMetadata{
			CustomBaseURL: "http://api.custom.com",
		}

		err := meta.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must use HTTPS scheme")
	})

	t.Run("too many tags", func(t *testing.T) {
		meta := &AccountMetadata{
			Tags: []string{"tag1", "tag2", "tag3", "tag4", "tag5", "tag6", "tag7", "tag8", "tag9", "tag10", "tag11"},
		}

		err := meta.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too many tags: max 10 allowed")
	})

	t.Run("tag too long", func(t *testing.T) {
		meta := &AccountMetadata{
			Tags: []string{"this-is-a-very-long-tag-name-that-exceeds-the-maximum-allowed-length-of-50-characters"},
		}

		err := meta.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tag[0] too long")
	})

	t.Run("empty tag", func(t *testing.T) {
		meta := &AccountMetadata{
			Tags: []string{"production", ""},
		}

		err := meta.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tag[1] is empty")
	})

	t.Run("notes too long", func(t *testing.T) {
		longNotes := string(make([]byte, 501))

		meta := &AccountMetadata{
			Notes: longNotes,
		}

		err := meta.Validate()

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "notes too long")
	})
}

func TestMaskSensitive(t *testing.T) {
	t.Run("mask proxy password", func(t *testing.T) {
		meta := &AccountMetadata{
			ProxyURL:     "socks5://user:password123@proxy.example.com:1080",
			ProxyEnabled: true,
			Region:       "us-east",
		}

		masked := meta.MaskSensitive()

		assert.Equal(t, "socks5://user:***@proxy.example.com:1080", masked.ProxyURL)
		assert.Equal(t, "us-east", masked.Region) // Other fields unchanged
		assert.True(t, masked.ProxyEnabled)
	})

	t.Run("mask proxy without password", func(t *testing.T) {
		meta := &AccountMetadata{
			ProxyURL: "socks5://proxy.example.com:1080",
		}

		masked := meta.MaskSensitive()

		assert.Equal(t, "socks5://proxy.example.com:1080", masked.ProxyURL) // Unchanged
	})

	t.Run("mask http proxy with auth", func(t *testing.T) {
		meta := &AccountMetadata{
			ProxyURL: "http://admin:secret@proxy.example.com:8080",
		}

		masked := meta.MaskSensitive()

		assert.Equal(t, "http://admin:***@proxy.example.com:8080", masked.ProxyURL)
	})

	t.Run("original metadata unchanged", func(t *testing.T) {
		original := &AccountMetadata{
			ProxyURL: "socks5://user:password@proxy.example.com:1080",
		}

		masked := original.MaskSensitive()

		// Verify original is unchanged
		assert.Equal(t, "socks5://user:password@proxy.example.com:1080", original.ProxyURL)
		// Verify masked is different
		assert.Equal(t, "socks5://user:***@proxy.example.com:1080", masked.ProxyURL)
	})
}

func TestIsEmpty(t *testing.T) {
	t.Run("empty metadata", func(t *testing.T) {
		meta := &AccountMetadata{}

		assert.True(t, meta.IsEmpty())
	})

	t.Run("non-empty metadata with proxy", func(t *testing.T) {
		meta := &AccountMetadata{
			ProxyURL: "socks5://proxy.example.com:1080",
		}

		assert.False(t, meta.IsEmpty())
	})

	t.Run("non-empty metadata with tags", func(t *testing.T) {
		meta := &AccountMetadata{
			Tags: []string{"production"},
		}

		assert.False(t, meta.IsEmpty())
	})
}
