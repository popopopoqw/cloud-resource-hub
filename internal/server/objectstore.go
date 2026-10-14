package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ObjectStore 对象存储抽象：R2 已配置时用 Cloudflare R2，
// 否则兜底到本地目录（开发/演示不断链；配置 R2 后自动切换）。
type ObjectStore interface {
	// Kind 返回存储类型（"r2" / "local"），供前端展示。
	Kind() string
	PutObject(key string, r io.Reader, contentType string, size int64) (int64, error)
	GetObject(key string) (io.ReadCloser, int64, string, error)
}

// LocalStore 本地文件对象存储（按 key 作为相对路径落盘）。
type LocalStore struct {
	root string
}

// NewLocalStore 创建本地存储。root 如 ./data。
func NewLocalStore(root string) *LocalStore {
	return &LocalStore{root: root}
}

// Kind 存储类型。
func (l *LocalStore) Kind() string { return "local" }

// PutObject 写入本地文件。
func (l *LocalStore) PutObject(key string, r io.Reader, _ string, _ int64) (int64, error) {
	if err := l.checkKey(key); err != nil {
		return 0, err
	}
	path := filepath.Join(l.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}

// GetObject 读取本地文件。
func (l *LocalStore) GetObject(key string) (io.ReadCloser, int64, string, error) {
	if err := l.checkKey(key); err != nil {
		return nil, 0, "", err
	}
	path := filepath.Join(l.root, filepath.FromSlash(key))
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, "", fmt.Errorf("对象不存在: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, "", err
	}
	ct := "application/octet-stream"
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pdf":
		ct = "application/pdf"
	case ".epub":
		ct = "application/epub+zip"
	case ".mobi", ".azw3":
		ct = "application/x-mobipocket-ebook"
	}
	return f, st.Size(), ct, nil
}

// checkKey 防路径穿越。
func (l *LocalStore) checkKey(key string) error {
	if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return fmt.Errorf("非法对象键: %s", key)
	}
	return nil
}
