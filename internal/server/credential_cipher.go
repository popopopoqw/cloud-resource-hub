package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const encryptedValuePrefix = "enc:v1:"

// credentialCipher 使用独立于 SQLite 的本地密钥加密敏感配置。
// 密钥文件仅允许当前系统用户读取，数据库泄露时不会同时暴露明文凭据。
type credentialCipher struct {
	aead cipher.AEAD
}

func newCredentialCipher(dataDir string) (*credentialCipher, error) {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "./data"
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建配置密钥目录失败: %w", err)
	}
	keyPath := filepath.Join(dataDir, "integration-settings.key")
	key, err := os.ReadFile(keyPath)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.WriteFile(keyPath, key, 0o600); err != nil {
			return nil, fmt.Errorf("写入配置密钥失败: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("读取配置密钥失败: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("配置密钥长度非法：期望 32 字节")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &credentialCipher{aead: aead}, nil
}

func (c *credentialCipher) Encrypt(plain string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("配置加密器不可用")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plain), nil)
	payload := append(nonce, sealed...)
	return encryptedValuePrefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (c *credentialCipher) Decrypt(value string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("配置加密器不可用")
	}
	if !strings.HasPrefix(value, encryptedValuePrefix) {
		return "", fmt.Errorf("敏感配置不是受支持的加密格式")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, encryptedValuePrefix))
	if err != nil {
		return "", fmt.Errorf("解码敏感配置失败: %w", err)
	}
	if len(payload) < c.aead.NonceSize() {
		return "", fmt.Errorf("敏感配置密文损坏")
	}
	nonce, sealed := payload[:c.aead.NonceSize()], payload[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("解密敏感配置失败: %w", err)
	}
	return string(plain), nil
}
