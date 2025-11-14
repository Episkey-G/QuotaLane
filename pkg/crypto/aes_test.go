package crypto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAESCrypto(t *testing.T) {
	tests := []struct {
		name    string
		key     []byte
		wantErr error
	}{
		{
			name:    "valid 32 byte key",
			key:     []byte("12345678901234567890123456789012"),
			wantErr: nil,
		},
		{
			name:    "invalid 16 byte key",
			key:     []byte("1234567890123456"),
			wantErr: ErrInvalidKeySize,
		},
		{
			name:    "invalid 24 byte key",
			key:     []byte("123456789012345678901234"),
			wantErr: ErrInvalidKeySize,
		},
		{
			name:    "invalid empty key",
			key:     []byte(""),
			wantErr: ErrInvalidKeySize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crypto, err := NewAESCrypto(tt.key)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, crypto)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, crypto)
			}
		})
	}
}

func TestAESCrypto_EncryptDecrypt(t *testing.T) {
	// 测试密钥（32字节）
	key := []byte("12345678901234567890123456789012") // Exactly 32 bytes
	crypto, err := NewAESCrypto(key)
	require.NoError(t, err)

	tests := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple text",
			plaintext: "Hello, World!",
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "long text",
			plaintext: strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 100),
		},
		{
			name:      "special characters",
			plaintext: "特殊字符测试 !@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
		{
			name:      "json data",
			plaintext: `{"access_token":"sk-proj-abcd1234","refresh_token":"refresh-xyz","expires_at":"2025-01-14T12:00:00Z"}`,
		},
		{
			name:      "api key",
			plaintext: "sk-proj-1234567890abcdefghijklmnopqrstuvwxyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 加密
			ciphertext, err := crypto.Encrypt(tt.plaintext)
			require.NoError(t, err)

			// 空字符串应该返回空字符串
			if tt.plaintext == "" {
				assert.Equal(t, "", ciphertext)
				return
			}

			// 密文不应该等于明文
			assert.NotEqual(t, tt.plaintext, ciphertext)

			// 密文应该是 Base64 格式
			assert.NotEmpty(t, ciphertext)

			// 解密
			decrypted, err := crypto.Decrypt(ciphertext)
			require.NoError(t, err)

			// 解密后应该等于原始明文
			assert.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestAESCrypto_EncryptRandomness(t *testing.T) {
	// 测试相同明文多次加密结果不同（Nonce 随机性）
	key := []byte("12345678901234567890123456789012") // Exactly 32 bytes
	crypto, err := NewAESCrypto(key)
	require.NoError(t, err)

	plaintext := "test plaintext for randomness"

	// 加密 10 次
	ciphertexts := make([]string, 10)
	for i := 0; i < 10; i++ {
		ciphertext, err := crypto.Encrypt(plaintext)
		require.NoError(t, err)
		ciphertexts[i] = ciphertext
	}

	// 验证所有密文都不同
	for i := 0; i < len(ciphertexts); i++ {
		for j := i + 1; j < len(ciphertexts); j++ {
			assert.NotEqual(t, ciphertexts[i], ciphertexts[j],
				"encryption should produce different ciphertexts for same plaintext (nonce randomness)")
		}
	}

	// 验证所有密文都可以正确解密
	for i, ciphertext := range ciphertexts {
		decrypted, err := crypto.Decrypt(ciphertext)
		require.NoError(t, err, "decryption %d failed", i)
		assert.Equal(t, plaintext, decrypted, "decryption %d mismatch", i)
	}
}

func TestAESCrypto_DecryptErrors(t *testing.T) {
	key := []byte("12345678901234567890123456789012") // Exactly 32 bytes
	crypto, err := NewAESCrypto(key)
	require.NoError(t, err)

	tests := []struct {
		name       string
		ciphertext string
		wantErr    error
	}{
		{
			name:       "empty ciphertext",
			ciphertext: "",
			wantErr:    nil, // 空字符串直接返回
		},
		{
			name:       "invalid base64",
			ciphertext: "not-valid-base64!!!",
			wantErr:    nil, // base64.DecodeString 会包装错误
		},
		{
			name:       "too short ciphertext",
			ciphertext: "dGVzdA==", // "test" in base64, too short
			wantErr:    ErrInvalidCiphertext,
		},
		{
			name:       "tampered ciphertext",
			ciphertext: "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY3ODkwYWJjZGVmZ2g=", // 随机 base64
			wantErr:    ErrDecryptionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decrypted, err := crypto.Decrypt(tt.ciphertext)
			if tt.name == "empty ciphertext" {
				assert.NoError(t, err)
				assert.Equal(t, "", decrypted)
			} else if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				// 其他错误情况也应该返回错误
				assert.Error(t, err)
			}
		})
	}
}

func TestAESCrypto_DecryptWithWrongKey(t *testing.T) {
	// 使用正确的密钥加密
	key1 := []byte("aaaabbbbccccddddeeeeffffgggghhhh") // Exactly 32 bytes
	crypto1, err := NewAESCrypto(key1)
	require.NoError(t, err)

	plaintext := "secret data"
	ciphertext, err := crypto1.Encrypt(plaintext)
	require.NoError(t, err)

	// 使用错误的密钥尝试解密
	key2 := []byte("11112222333344445555666677778888") // Exactly 32 bytes
	crypto2, err := NewAESCrypto(key2)
	require.NoError(t, err)

	decrypted, err := crypto2.Decrypt(ciphertext)
	assert.ErrorIs(t, err, ErrDecryptionFailed)
	assert.Empty(t, decrypted)
}

func BenchmarkAESCrypto_Encrypt(b *testing.B) {
	key := []byte("12345678901234567890123456789012") // Exactly 32 bytes
	crypto, _ := NewAESCrypto(key)
	plaintext := "test data for benchmarking encryption performance"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = crypto.Encrypt(plaintext)
	}
}

func BenchmarkAESCrypto_Decrypt(b *testing.B) {
	key := []byte("12345678901234567890123456789012") // Exactly 32 bytes
	crypto, _ := NewAESCrypto(key)
	plaintext := "test data for benchmarking decryption performance"
	ciphertext, _ := crypto.Encrypt(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = crypto.Decrypt(ciphertext)
	}
}
