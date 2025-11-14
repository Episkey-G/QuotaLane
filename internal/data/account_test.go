package data

import (
	"database/sql/driver"
	"testing"
	"time"

	v1 "QuotaLane/api/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAccountProvider_ScanValue tests enum scanning and value conversion.
func TestAccountProvider_ScanValue(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantValue AccountProvider
		wantErr   bool
	}{
		{
			name:      "scan from string",
			input:     "claude-console",
			wantValue: ProviderClaudeConsole,
			wantErr:   false,
		},
		{
			name:      "scan from bytes",
			input:     []byte("openai-responses"),
			wantValue: ProviderOpenAIResponses,
			wantErr:   false,
		},
		{
			name:      "scan from nil",
			input:     nil,
			wantValue: "",
			wantErr:   false,
		},
		{
			name:      "scan from invalid type",
			input:     123,
			wantValue: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p AccountProvider
			err := p.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantValue, p)
			}
		})
	}

	// Test Value() method
	t.Run("Value returns string", func(t *testing.T) {
		p := ProviderClaudeConsole
		val, err := p.Value()
		assert.NoError(t, err)
		assert.Equal(t, driver.Value("claude-console"), val)
	})
}

// TestAccountStatus_ScanValue tests status enum scanning and value conversion.
func TestAccountStatus_ScanValue(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantValue AccountStatus
		wantErr   bool
	}{
		{
			name:      "scan from string",
			input:     "active",
			wantValue: StatusActive,
			wantErr:   false,
		},
		{
			name:      "scan from bytes",
			input:     []byte("inactive"),
			wantValue: StatusInactive,
			wantErr:   false,
		},
		{
			name:      "scan from nil",
			input:     nil,
			wantValue: "",
			wantErr:   false,
		},
		{
			name:      "scan from invalid type",
			input:     456,
			wantValue: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s AccountStatus
			err := s.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantValue, s)
			}
		})
	}

	// Test Value() method
	t.Run("Value returns string", func(t *testing.T) {
		s := StatusActive
		val, err := s.Value()
		assert.NoError(t, err)
		assert.Equal(t, driver.Value("active"), val)
	})
}

// TestProviderConversion tests Proto <-> Data provider conversion.
func TestProviderConversion(t *testing.T) {
	tests := []struct {
		name      string
		proto     v1.AccountProvider
		data      AccountProvider
		shouldMap bool
	}{
		{
			name:      "CLAUDE_CONSOLE",
			proto:     v1.AccountProvider_CLAUDE_CONSOLE,
			data:      ProviderClaudeConsole,
			shouldMap: true,
		},
		{
			name:      "OPENAI_RESPONSES",
			proto:     v1.AccountProvider_OPENAI_RESPONSES,
			data:      ProviderOpenAIResponses,
			shouldMap: true,
		},
		{
			name:      "CLAUDE_OFFICIAL",
			proto:     v1.AccountProvider_CLAUDE_OFFICIAL,
			data:      ProviderClaudeOfficial,
			shouldMap: true,
		},
		{
			name:      "BEDROCK",
			proto:     v1.AccountProvider_BEDROCK,
			data:      ProviderBedrock,
			shouldMap: true,
		},
		{
			name:      "CCR",
			proto:     v1.AccountProvider_CCR,
			data:      ProviderCCR,
			shouldMap: true,
		},
		{
			name:      "DROID",
			proto:     v1.AccountProvider_DROID,
			data:      ProviderDroid,
			shouldMap: true,
		},
		{
			name:      "GEMINI",
			proto:     v1.AccountProvider_GEMINI,
			data:      ProviderGemini,
			shouldMap: true,
		},
		{
			name:      "AZURE_OPENAI",
			proto:     v1.AccountProvider_AZURE_OPENAI,
			data:      ProviderAzureOpenAI,
			shouldMap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" Proto->Data", func(t *testing.T) {
			result := ProviderFromProto(tt.proto)
			if tt.shouldMap {
				assert.Equal(t, tt.data, result)
			}
		})

		t.Run(tt.name+" Data->Proto", func(t *testing.T) {
			result := ProviderToProto(tt.data)
			if tt.shouldMap {
				assert.Equal(t, tt.proto, result)
			}
		})
	}
}

// TestStatusConversion tests Proto <-> Data status conversion.
func TestStatusConversion(t *testing.T) {
	tests := []struct {
		name  string
		proto v1.AccountStatus
		data  AccountStatus
	}{
		{
			name:  "ACTIVE",
			proto: v1.AccountStatus_ACCOUNT_ACTIVE,
			data:  StatusActive,
		},
		{
			name:  "INACTIVE",
			proto: v1.AccountStatus_ACCOUNT_INACTIVE,
			data:  StatusInactive,
		},
		{
			name:  "ERROR",
			proto: v1.AccountStatus_ACCOUNT_ERROR,
			data:  StatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" Proto->Data", func(t *testing.T) {
			result := StatusFromProto(tt.proto)
			assert.Equal(t, tt.data, result)
		})

		t.Run(tt.name+" Data->Proto", func(t *testing.T) {
			result := StatusToProto(tt.data)
			assert.Equal(t, tt.proto, result)
		})
	}
}

// TestAccount_TableName tests GORM table name.
func TestAccount_TableName(t *testing.T) {
	account := Account{}
	assert.Equal(t, "api_accounts", account.TableName())
}

// TestAccount_ToProto tests GORM model to Proto conversion.
func TestAccount_ToProto(t *testing.T) {
	now := time.Now()
	account := &Account{
		ID:                 1,
		Name:               "Test Account",
		Provider:           ProviderClaudeConsole,
		APIKeyEncrypted:    "encrypted-api-key",
		OAuthDataEncrypted: "encrypted-oauth-data",
		RpmLimit:           50,
		TpmLimit:           100000,
		HealthScore:        95,
		IsCircuitBroken:    false,
		Status:             StatusActive,
		Metadata:           `{"region":"us-east-1"}`,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	proto := account.ToProto()

	assert.Equal(t, int64(1), proto.Id)
	assert.Equal(t, "Test Account", proto.Name)
	assert.Equal(t, v1.AccountProvider_CLAUDE_CONSOLE, proto.Provider)
	assert.Equal(t, "encrypted-api-key", proto.ApiKeyEncrypted)
	assert.Equal(t, "encrypted-oauth-data", proto.OauthDataEncrypted)
	assert.Equal(t, int32(50), proto.RpmLimit)
	assert.Equal(t, int32(100000), proto.TpmLimit)
	assert.Equal(t, int32(95), proto.HealthScore)
	assert.False(t, proto.IsCircuitBroken)
	assert.Equal(t, v1.AccountStatus_ACCOUNT_ACTIVE, proto.Status)
	assert.Equal(t, `{"region":"us-east-1"}`, proto.Metadata)
	assert.NotNil(t, proto.CreatedAt)
	assert.NotNil(t, proto.UpdatedAt)
}

// TestAccount_MaskSensitiveData tests sensitive data masking.
func TestAccount_MaskSensitiveData(t *testing.T) {
	tests := []struct {
		name              string
		apiKey            string
		oauthData         string
		expectedAPIKey    string
		expectedOAuthData string
	}{
		{
			name:              "mask long API key",
			apiKey:            "sk-proj-1234567890abcdef",
			oauthData:         `{"token":"secret"}`,
			expectedAPIKey:    "sk-p****cdef",
			expectedOAuthData: "[ENCRYPTED]",
		},
		{
			name:              "mask short API key",
			apiKey:            "short",
			oauthData:         "",
			expectedAPIKey:    "short",
			expectedOAuthData: "",
		},
		{
			name:              "empty values",
			apiKey:            "",
			oauthData:         "",
			expectedAPIKey:    "",
			expectedOAuthData: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				APIKeyEncrypted:    tt.apiKey,
				OAuthDataEncrypted: tt.oauthData,
			}

			account.MaskSensitiveData()

			assert.Equal(t, tt.expectedAPIKey, account.APIKeyEncrypted)
			assert.Equal(t, tt.expectedOAuthData, account.OAuthDataEncrypted)
		})
	}
}

// TestMaskAPIKey tests the standalone API key masking function.
func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "long key",
			input:    "sk-proj-1234567890abcdef",
			expected: "sk-p****cdef",
		},
		{
			name:     "short key (8 chars)",
			input:    "12345678",
			expected: "********",
		},
		{
			name:     "very short key",
			input:    "short",
			expected: "*****",
		},
		{
			name:     "empty key",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestValidateMetadataJSON tests JSON metadata validation.
func TestValidateMetadataJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid JSON object",
			input:   `{"region":"us-east-1","proxy":"http://proxy:8080"}`,
			wantErr: false,
		},
		{
			name:    "valid JSON array",
			input:   `["tag1","tag2"]`,
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "not a JSON",
			input:   "plain text",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetadataJSON(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestAccountFilter tests the AccountFilter struct.
func TestAccountFilter(t *testing.T) {
	t.Run("creates valid filter", func(t *testing.T) {
		provider := v1.AccountProvider_CLAUDE_CONSOLE
		status := v1.AccountStatus_ACCOUNT_ACTIVE

		filter := &AccountFilter{
			Page:     1,
			PageSize: 20,
			Provider: ProviderFromProto(provider),
			Status:   StatusFromProto(status),
		}

		assert.Equal(t, int32(1), filter.Page)
		assert.Equal(t, int32(20), filter.PageSize)
		assert.Equal(t, ProviderClaudeConsole, filter.Provider)
		assert.Equal(t, StatusActive, filter.Status)
	})
}

// TestUnspecifiedProviderConversion tests unspecified enum handling.
func TestUnspecifiedProviderConversion(t *testing.T) {
	t.Run("UNSPECIFIED proto to data", func(t *testing.T) {
		result := ProviderFromProto(v1.AccountProvider_ACCOUNT_PROVIDER_UNSPECIFIED)
		assert.Equal(t, AccountProvider(""), result)
	})

	t.Run("empty data to proto", func(t *testing.T) {
		result := ProviderToProto("")
		assert.Equal(t, v1.AccountProvider_ACCOUNT_PROVIDER_UNSPECIFIED, result)
	})
}

// TestUnspecifiedStatusConversion tests unspecified status handling.
func TestUnspecifiedStatusConversion(t *testing.T) {
	t.Run("UNSPECIFIED proto to data", func(t *testing.T) {
		result := StatusFromProto(v1.AccountStatus_ACCOUNT_STATUS_UNSPECIFIED)
		// StatusFromProto returns StatusActive as default for UNSPECIFIED
		assert.Equal(t, StatusActive, result)
	})

	t.Run("empty data to proto", func(t *testing.T) {
		result := StatusToProto("")
		assert.Equal(t, v1.AccountStatus_ACCOUNT_STATUS_UNSPECIFIED, result)
	})
}

// TestAccount_ToProto_AllProviders tests all provider conversions.
func TestAccount_ToProto_AllProviders(t *testing.T) {
	providers := []struct {
		data  AccountProvider
		proto v1.AccountProvider
	}{
		{ProviderClaudeOfficial, v1.AccountProvider_CLAUDE_OFFICIAL},
		{ProviderClaudeConsole, v1.AccountProvider_CLAUDE_CONSOLE},
		{ProviderBedrock, v1.AccountProvider_BEDROCK},
		{ProviderCCR, v1.AccountProvider_CCR},
		{ProviderDroid, v1.AccountProvider_DROID},
		{ProviderGemini, v1.AccountProvider_GEMINI},
		{ProviderOpenAIResponses, v1.AccountProvider_OPENAI_RESPONSES},
		{ProviderAzureOpenAI, v1.AccountProvider_AZURE_OPENAI},
	}

	for _, p := range providers {
		t.Run(string(p.data), func(t *testing.T) {
			account := &Account{
				ID:       1,
				Name:     "Test",
				Provider: p.data,
				Status:   StatusActive,
			}

			proto := account.ToProto()
			assert.Equal(t, p.proto, proto.Provider)
		})
	}
}

// TestAccount_ToProto_AllStatuses tests all status conversions.
func TestAccount_ToProto_AllStatuses(t *testing.T) {
	statuses := []struct {
		data  AccountStatus
		proto v1.AccountStatus
	}{
		{StatusActive, v1.AccountStatus_ACCOUNT_ACTIVE},
		{StatusInactive, v1.AccountStatus_ACCOUNT_INACTIVE},
		{StatusError, v1.AccountStatus_ACCOUNT_ERROR},
	}

	for _, s := range statuses {
		t.Run(string(s.data), func(t *testing.T) {
			account := &Account{
				ID:       1,
				Name:     "Test",
				Provider: ProviderClaudeConsole,
				Status:   s.data,
			}

			proto := account.ToProto()
			assert.Equal(t, s.proto, proto.Status)
		})
	}
}

// TestAccount_MaskSensitiveData_EdgeCases tests edge cases for masking.
func TestAccount_MaskSensitiveData_EdgeCases(t *testing.T) {
	t.Run("exactly 8 characters", func(t *testing.T) {
		account := &Account{
			APIKeyEncrypted: "12345678",
		}
		account.MaskSensitiveData()
		assert.Equal(t, "12345678", account.APIKeyEncrypted) // Not masked (needs > 8)
	})

	t.Run("9 characters", func(t *testing.T) {
		account := &Account{
			APIKeyEncrypted: "123456789",
		}
		account.MaskSensitiveData()
		assert.Equal(t, "1234****6789", account.APIKeyEncrypted)
	})

	t.Run("only OAuth data", func(t *testing.T) {
		account := &Account{
			OAuthDataEncrypted: `{"access_token":"secret"}`,
		}
		account.MaskSensitiveData()
		assert.Equal(t, "[ENCRYPTED]", account.OAuthDataEncrypted)
	})
}

// TestProviderFromProto_AllCases tests all provider enum conversions from proto.
func TestProviderFromProto_AllCases(t *testing.T) {
	// Test all valid providers
	require.Equal(t, ProviderClaudeOfficial, ProviderFromProto(v1.AccountProvider_CLAUDE_OFFICIAL))
	require.Equal(t, ProviderClaudeConsole, ProviderFromProto(v1.AccountProvider_CLAUDE_CONSOLE))
	require.Equal(t, ProviderBedrock, ProviderFromProto(v1.AccountProvider_BEDROCK))
	require.Equal(t, ProviderCCR, ProviderFromProto(v1.AccountProvider_CCR))
	require.Equal(t, ProviderDroid, ProviderFromProto(v1.AccountProvider_DROID))
	require.Equal(t, ProviderGemini, ProviderFromProto(v1.AccountProvider_GEMINI))
	require.Equal(t, ProviderOpenAIResponses, ProviderFromProto(v1.AccountProvider_OPENAI_RESPONSES))
	require.Equal(t, ProviderAzureOpenAI, ProviderFromProto(v1.AccountProvider_AZURE_OPENAI))
}

// TestProviderToProto_AllCases tests all provider enum conversions to proto.
func TestProviderToProto_AllCases(t *testing.T) {
	// Test all valid providers
	require.Equal(t, v1.AccountProvider_CLAUDE_OFFICIAL, ProviderToProto(ProviderClaudeOfficial))
	require.Equal(t, v1.AccountProvider_CLAUDE_CONSOLE, ProviderToProto(ProviderClaudeConsole))
	require.Equal(t, v1.AccountProvider_BEDROCK, ProviderToProto(ProviderBedrock))
	require.Equal(t, v1.AccountProvider_CCR, ProviderToProto(ProviderCCR))
	require.Equal(t, v1.AccountProvider_DROID, ProviderToProto(ProviderDroid))
	require.Equal(t, v1.AccountProvider_GEMINI, ProviderToProto(ProviderGemini))
	require.Equal(t, v1.AccountProvider_OPENAI_RESPONSES, ProviderToProto(ProviderOpenAIResponses))
	require.Equal(t, v1.AccountProvider_AZURE_OPENAI, ProviderToProto(ProviderAzureOpenAI))
}

// TestStatusFromProto_AllCases tests all status enum conversions from proto.
func TestStatusFromProto_AllCases(t *testing.T) {
	require.Equal(t, StatusActive, StatusFromProto(v1.AccountStatus_ACCOUNT_ACTIVE))
	require.Equal(t, StatusInactive, StatusFromProto(v1.AccountStatus_ACCOUNT_INACTIVE))
	require.Equal(t, StatusError, StatusFromProto(v1.AccountStatus_ACCOUNT_ERROR))
}

// TestStatusToProto_AllCases tests all status enum conversions to proto.
func TestStatusToProto_AllCases(t *testing.T) {
	require.Equal(t, v1.AccountStatus_ACCOUNT_ACTIVE, StatusToProto(StatusActive))
	require.Equal(t, v1.AccountStatus_ACCOUNT_INACTIVE, StatusToProto(StatusInactive))
	require.Equal(t, v1.AccountStatus_ACCOUNT_ERROR, StatusToProto(StatusError))
}
