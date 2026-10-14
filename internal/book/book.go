// Package book 定义书库 Provider 抽象：搜索、详情、下载。
// 与 pan.Provider 平行；实现方包括 Mock（演示）与 Z-Library 客户端。
package book

import (
	"context"
	"io"

	"resource-center/internal/model"
)

// Provider 书库提供方接口。
type Provider interface {
	// Probe 验证书库服务是否可访问。
	Probe(ctx context.Context) error
	// Search 按关键词搜索书籍。
	Search(ctx context.Context, q model.BookQuery) (*model.BookSearchResult, error)
	// Detail 书籍详情。
	Detail(ctx context.Context, bookID string) (*model.Book, error)
	// Download 打开书籍文件流。调用方负责关闭；size 为文件大小（未知时 -1）。
	Download(ctx context.Context, bookID string) (rc io.ReadCloser, size int64, format string, err error)
}
