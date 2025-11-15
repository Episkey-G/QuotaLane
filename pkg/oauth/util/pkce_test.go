package util

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCodeVerifier(t *testing.T) {
	tests := []struct {
		name       string
		sizeBytes  int
		encoding   string
		wantLength int // expected output length
		wantErr    bool
	}{
		{
			name:       "Claude PKCE - 32 bytes base64url",
			sizeBytes:  32,
			encoding:   "base64url",
			wantLength: 43, // 32 bytes base64url ≈ 43 chars (without padding)
			wantErr:    false,
		},
		{
			name:       "Codex PKCE - 64 bytes hex",
			sizeBytes:  64,
			encoding:   "hex",
			wantLength: 128, // 64 bytes hex = 128 chars
			wantErr:    false,
		},
		{
			name:      "Unsupported encoding",
			sizeBytes: 32,
			encoding:  "invalid",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codeVerifier, err := GenerateCodeVerifier(tt.sizeBytes, tt.encoding)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantLength, len(codeVerifier), "code verifier length mismatch")

			// Verify encoding is valid
			switch tt.encoding {
			case "base64url":
				_, err := base64.RawURLEncoding.DecodeString(codeVerifier)
				assert.NoError(t, err, "invalid base64url encoding")
			case "hex":
				_, err := hex.DecodeString(codeVerifier)
				assert.NoError(t, err, "invalid hex encoding")
			}

			// Verify randomness: generate again and ensure different
			codeVerifier2, err := GenerateCodeVerifier(tt.sizeBytes, tt.encoding)
			require.NoError(t, err)
			assert.NotEqual(t, codeVerifier, codeVerifier2, "code verifiers should be random")
		})
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	tests := []struct {
		name          string
		codeVerifier  string
		wantChallenge string // expected SHA256 base64url output
	}{
		{
			name:          "Known test vector",
			codeVerifier:  "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk",
			wantChallenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", // RFC 7636 example
		},
		{
			name:          "Empty verifier",
			codeVerifier:  "",
			wantChallenge: "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU", // SHA256 of empty string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			challenge := GenerateCodeChallenge(tt.codeVerifier)
			assert.Equal(t, tt.wantChallenge, challenge)

			// Verify it's valid base64url
			_, err := base64.RawURLEncoding.DecodeString(challenge)
			assert.NoError(t, err, "challenge should be valid base64url")

			// Verify it's SHA256 hash
			hash := sha256.Sum256([]byte(tt.codeVerifier))
			expectedChallenge := base64.RawURLEncoding.EncodeToString(hash[:])
			assert.Equal(t, expectedChallenge, challenge)
		})
	}
}

func TestGenerateState(t *testing.T) {
	t.Run("State generation", func(t *testing.T) {
		state, err := GenerateState()
		require.NoError(t, err)

		// State should be 64 chars (32 bytes hex)
		assert.Equal(t, 64, len(state), "state should be 64 hex chars")

		// Verify valid hex encoding
		_, err = hex.DecodeString(state)
		assert.NoError(t, err, "state should be valid hex")

		// Verify randomness
		state2, err := GenerateState()
		require.NoError(t, err)
		assert.NotEqual(t, state, state2, "states should be random")
	})
}

func TestGenerateSessionID(t *testing.T) {
	t.Run("SessionID generation", func(t *testing.T) {
		sessionID, err := GenerateSessionID()
		require.NoError(t, err)

		// SessionID should be 43 chars (32 bytes base64url)
		assert.Equal(t, 43, len(sessionID), "sessionID should be 43 base64url chars")

		// Verify valid base64url encoding
		decoded, err := base64.RawURLEncoding.DecodeString(sessionID)
		assert.NoError(t, err, "sessionID should be valid base64url")
		assert.Equal(t, 32, len(decoded), "sessionID should decode to 32 bytes")

		// Verify randomness
		sessionID2, err := GenerateSessionID()
		require.NoError(t, err)
		assert.NotEqual(t, sessionID, sessionID2, "sessionIDs should be random")
	})
}

// Benchmark tests
func BenchmarkGenerateCodeVerifier(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateCodeVerifier(32, "base64url")
	}
}

func BenchmarkGenerateCodeChallenge(b *testing.B) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateCodeChallenge(verifier)
	}
}

func BenchmarkGenerateState(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateState()
	}
}
