package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeField_Password(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "password field",
			key:      "password",
			value:    "mysecretpassword123",
			expected: "myse***********d123",
		},
		{
			name:     "passwd field",
			key:      "passwd",
			value:    "testpass",
			expected: "t******s",
		},
		{
			name:     "user_password field",
			key:      "user_password",
			value:    "p@ssw0rd!",
			expected: "p@ss*0rd!",
		},
		{
			name:     "PASSWORD uppercase",
			key:      "PASSWORD",
			value:    "SecretPass123",
			expected: "Secr*****s123",
		},
		{
			name:     "short password",
			key:      "pwd",
			value:    "abc",
			expected: "a*c",
		},
		{
			name:     "very short password",
			key:      "pwd",
			value:    "ab",
			expected: "**",
		},
		{
			name:     "single char password",
			key:      "pwd",
			value:    "a",
			expected: "*",
		},
		{
			name:     "empty password",
			key:      "password",
			value:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeField(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeField_Token(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "api_key field",
			key:      "api_key",
			value:    "sk-1234567890abcdefghij",
			expected: "sk-1***************ghij",
		},
		{
			name:     "access_token field",
			key:      "access_token",
			value:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "eyJh****************************VCJ9",
		},
		{
			name:     "token field",
			key:      "token",
			value:    "abc123xyz789",
			expected: "abc1****z789",
		},
		{
			name:     "authorization header",
			key:      "Authorization",
			value:    "Bearer token123456",
			expected: "Bear**********3456",
		},
		{
			name:     "secret field",
			key:      "secret",
			value:    "my_secret_key_here",
			expected: "my_s**********here",
		},
		{
			name:     "apikey no underscore",
			key:      "apikey",
			value:    "key12345",
			expected: "k******5",
		},
		{
			name:     "private_key field",
			key:      "private_key",
			value:    "-----BEGIN PRIVATE KEY-----",
			expected: "----*******************----",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeField(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeField_Email(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "email field",
			key:      "email",
			value:    "user@example.com",
			expected: "use***@example.com",
		},
		{
			name:     "user_email field",
			key:      "user_email",
			value:    "john.doe@company.org",
			expected: "joh***@company.org",
		},
		{
			name:     "short email",
			key:      "email",
			value:    "ab@test.com",
			expected: "a*@test.com",
		},
		{
			name:     "single char email",
			key:      "email",
			value:    "a@test.com",
			expected: "a@test.com",
		},
		{
			name:     "invalid email no at",
			key:      "email",
			value:    "notanemail",
			expected: "**********",
		},
		{
			name:     "invalid email multiple at",
			key:      "email",
			value:    "user@@example.com",
			expected: "*****************",
		},
		{
			name:     "empty email",
			key:      "email",
			value:    "",
			expected: "",
		},
		{
			name:     "mail field",
			key:      "mail",
			value:    "admin@domain.com",
			expected: "adm***@domain.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeField(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeField_NonSensitive(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "normal field",
			key:      "username",
			value:    "john_doe",
			expected: "john_doe",
		},
		{
			name:     "id field",
			key:      "user_id",
			value:    "12345",
			expected: "12345",
		},
		{
			name:     "name field",
			key:      "name",
			value:    "John Doe",
			expected: "John Doe",
		},
		{
			name:     "message field",
			key:      "message",
			value:    "Hello world",
			expected: "Hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeField(tt.key, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeField_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"PASSWORD uppercase", "PASSWORD", "secret123"},
		{"Password mixed", "Password", "secret123"},
		{"password lowercase", "password", "secret123"},
		{"API_KEY uppercase", "API_KEY", "key123456"},
		{"Api_Key mixed", "Api_Key", "key123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeField(tt.key, tt.value)
			// All should be sanitized regardless of case
			assert.NotEqual(t, tt.value, result)
			assert.Contains(t, result, "*")
		})
	}
}

func TestSanitizeToken_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "8 char string boundary",
			value:    "12345678",
			expected: "1******8",
		},
		{
			name:     "9 char string",
			value:    "123456789",
			expected: "1234*6789",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "",
		},
		{
			name:     "single char",
			value:    "a",
			expected: "*",
		},
		{
			name:     "two chars",
			value:    "ab",
			expected: "**",
		},
		{
			name:     "three chars",
			value:    "abc",
			expected: "a*c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeToken(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeEmail_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "no local part",
			value:    "@example.com",
			expected: "@example.com",
		},
		{
			name:     "very long local part",
			value:    "verylongemailaddress@example.com",
			expected: "ver***@example.com",
		},
		{
			name:     "3 char local part boundary",
			value:    "abc@example.com",
			expected: "a**@example.com",
		},
		{
			name:     "special chars in email",
			value:    "user+tag@example.com",
			expected: "use***@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeEmail(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeField_MixedCaseKeys(t *testing.T) {
	// Test that key matching is case-insensitive
	sensitiveKeys := []string{
		"Password", "PASSWORD", "password",
		"ApiKey", "API_KEY", "api_key",
		"Token", "TOKEN", "token",
		"Secret", "SECRET", "secret",
		"Email", "EMAIL", "email",
	}

	for _, key := range sensitiveKeys {
		t.Run(key, func(t *testing.T) {
			result := SanitizeField(key, "sensit ivevalue123")
			// All should be masked
			assert.Contains(t, result, "*")
			assert.NotEqual(t, "sensitivevalue123", result)
		})
	}
}
