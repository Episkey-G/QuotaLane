package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrInvalidKeySize 密钥长度无效错误
	ErrInvalidKeySize = errors.New("encryption key must be 32 bytes (256 bits)")
	// ErrInvalidCiphertext 密文格式无效错误
	ErrInvalidCiphertext = errors.New("invalid ciphertext: too short or malformed")
	// ErrDecryptionFailed 解密失败错误
	ErrDecryptionFailed = errors.New("decryption failed: authentication failed")
)

// AESCrypto AES-256-GCM 加密服务
type AESCrypto struct {
	key []byte
}

// NewAESCrypto 创建 AES 加密服务
// key 必须为 32 字节（256 位）
func NewAESCrypto(key []byte) (*AESCrypto, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidKeySize, len(key))
	}

	return &AESCrypto{
		key: key,
	}, nil
}

// Encrypt 使用 AES-256-GCM 加密明文
// 返回 Base64 编码的密文（格式：nonce(12字节) + ciphertext + tag(16字节)）
func (a *AESCrypto) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil // 空字符串直接返回
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// 创建 GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// 生成随机 nonce（12 字节）
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 加密（nonce + ciphertext + tag）
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Base64 编码
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return encoded, nil
}

// Decrypt 使用 AES-256-GCM 解密密文
// ciphertext 为 Base64 编码的密文
func (a *AESCrypto) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil // 空字符串直接返回
	}

	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// 创建 GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// 验证密文长度（至少包含 nonce + tag）
	nonceSize := gcm.NonceSize()
	if len(decoded) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	// 提取 nonce 和 ciphertext
	nonce, encrypted := decoded[:nonceSize], decoded[nonceSize:]

	// 解密并验证
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return string(plaintext), nil
}
