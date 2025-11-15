package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// GenerateCodeVerifier 生成 PKCE code_verifier
// Claude: 32 字节 → base64url 编码（约 43 字符）
// Codex: 64 字节 → hex 编码（128 字符）
func GenerateCodeVerifier(sizeBytes int, encoding string) (string, error) {
	bytes := make([]byte, sizeBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	switch encoding {
	case "base64url":
		return base64.RawURLEncoding.EncodeToString(bytes), nil
	case "hex":
		return hex.EncodeToString(bytes), nil
	default:
		return "", fmt.Errorf("unsupported encoding: %s", encoding)
	}
}

// GenerateCodeChallenge 生成 PKCE code_challenge
// SHA256(code_verifier) → base64url 编码
func GenerateCodeChallenge(codeVerifier string) string {
	hash := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// GenerateState 生成 CSRF 防护 state 参数
// 32 字节随机数 → hex 编码（64 字符）
func GenerateState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateSessionID 生成会话 ID
// 32 字节随机数 → base64url 编码
func GenerateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
