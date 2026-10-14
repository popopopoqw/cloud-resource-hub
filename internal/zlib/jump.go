package zlib

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"resource-center/internal/model"
)

// Jump 浏览器跳转模式：Z-Library 的 DiamWall 反爬对服务端直连封锁
// （TLS 指纹 + JS 挑战 + cookie 绑定），书库降级为站内入口 + 新窗口
// 跳转原站搜索。不发起任何对 z-lib 的服务端请求。
type Jump struct {
	BaseURL string
}

// NewJump 创建跳转模式。
func NewJump(baseURL string) *Jump { return &Jump{BaseURL: baseURL} }

// Probe 验证跳转站点的网络可达性；4xx 也表示站点已响应。
func (j *Jump) Probe(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, j.BaseURL, nil)
	if err != nil {
		return err
	}
	// 书库域名会跳转到地区/个人入口；首个 3xx 已能证明入口可达，
	// 不继续跟随可避免镜像域名之间循环重定向导致误报异常。
	client := &http.Client{
		Timeout:       8 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("书库站点 HTTP %d", resp.StatusCode)
	}
	return nil
}

// Search 实现 book.Provider（跳转模式下返回明确错误，前端不调用）。
func (j *Jump) Search(_ context.Context, _ model.BookQuery) (*model.BookSearchResult, error) {
	return nil, fmt.Errorf("跳转模式：请在页面中打开 Z-Library 搜索")
}

// Detail 实现 book.Provider。
func (j *Jump) Detail(_ context.Context, _ string) (*model.Book, error) {
	return nil, fmt.Errorf("跳转模式不支持站内详情")
}

// Download 实现 book.Provider。
func (j *Jump) Download(_ context.Context, _ string) (io.ReadCloser, int64, string, error) {
	return nil, 0, "", fmt.Errorf("跳转模式不支持站内下载")
}

// SearchURL 构造站外搜索链接（语言/格式为 z-lib 查询参数）。
func (j *Jump) SearchURL(keyword, lang, ext string) string {
	u := j.BaseURL + "/s/" + pathEscape(keyword)
	sep := "?"
	if lang != "" {
		u += sep + "languages=" + queryEscape(lang)
		sep = "&"
	}
	if ext != "" {
		u += sep + "extensions=" + queryEscape(ext)
	}
	return u
}

func pathEscape(s string) string  { return url.PathEscape(strings.TrimSpace(s)) }
func queryEscape(s string) string { return url.QueryEscape(strings.TrimSpace(s)) }
