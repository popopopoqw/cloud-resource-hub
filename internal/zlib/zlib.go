// Package zlib 实现 Z-Library 书库客户端：账号密码自动登录 + 搜索/详情/下载。
//
// ⚠️ 待联调：真实凭据（ZLIB_EMAIL/ZLIB_PASSWORD）提供后需要用实际页面校准
// 登录端点与 HTML 选择器（见 selectors 与 login 流程中的 TODO）。
// 当前实现保证完整代码路径可编译、可执行，选择器集中在 selectors 便于调整。
//
// 站点行为（2026-09 调研）：
//   - z-lib.sk 为统一登录授权页，登录成功后 302 到个人专属域名
//   - 搜索  GET {home}/s/{关键词}?page=N（可选 &language= &extensions= &sort=）
//   - 详情  GET {home}/book/{id}/{slug}
//   - 下载  详情页解析下载入口后跟随（免费账号有每日额度限制）
package zlib

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"resource-center/internal/model"
)

// Options 客户端配置。
type Options struct {
	BaseURL  string // 授权登录站点，如 https://z-lib.sk
	Email    string
	Password string
}

// Client Z-Library 客户端（并发安全；登录态惰性获取并缓存）。
type Client struct {
	opt    Options
	client *http.Client

	mu       sync.Mutex
	home     string // 登录后的个人域名（跟随 302 得到）
	loggedIn bool
	lastErr  error
}

// New 创建客户端。
func New(opt Options) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		opt: opt,
		client: &http.Client{
			Timeout:   60 * time.Second,
			Jar:       jar,
			Transport: newStealthTransport(), // Chrome TLS 指纹，绕过 JA3 检测
			// 登录 POST 的 302 不自动跟随：需要读 Location 判定成功/失败；
			// 其余请求照常跟随（跨镜像 307 / 登录后跳个人域名）。
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 0 && via[0].Method == http.MethodPost &&
					strings.HasSuffix(via[0].URL.Path, "/login") {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

// Probe 验证直连书库站点的网络可达性；4xx 也表示站点已响应。
func (c *Client) Probe(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.opt.BaseURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("书库站点 HTTP %d", resp.StatusCode)
	}
	return nil
}

// selectors HTML 解析选择器（联调时按实际页面校准）。
// 说明：Z-Library 结果页为表格行结构，历史多版域名保持 .resRow/.bookRow 结构。
var (
	reRow      = regexp.MustCompile(`(?s)<tr[^>]*class="[^"]*resRow[^"]*"[^>]*>(.*?)</tr>`)
	reBookHref = regexp.MustCompile(`href="/book/([0-9]+)/([^"]+)"`)
	reTitle    = regexp.MustCompile(`<h3[^>]*>\s*<a[^>]*>(.*?)</a>`)
	reAuthor   = regexp.MustCompile(`<a[^>]*class="[^"]*author[^"]*"[^>]*>(.*?)</a>`)
	reCover    = regexp.MustCompile(`<img[^>]+src="([^"]+)"[^>]*data-cover`)
	reAttr     = regexp.MustCompile(`<td[^>]*class="[^"]*(lang|ext|size|year|rating)[^"]*"[^>]*>(.*?)</td>`)
	reDesc     = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*description[^"]*"[^>]*>(.*?)</div>`)
	reNextPage = regexp.MustCompile(`href="[^"]*page=(\d+)"[^>]*>下一页|href="[^"]*page=(\d+)"[^>]*>Next`)
)

// Search 搜索书籍。
func (c *Client) Search(ctx context.Context, q model.BookQuery) (*model.BookSearchResult, error) {
	home, err := c.ensureLogin(ctx)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/s/%s?page=%d", home, url.PathEscape(q.Keyword), q.Page)
	if q.Lang != "" {
		u += "&language=" + url.QueryEscape(q.Lang)
	}
	if q.Ext != "" {
		u += "&extensions=" + url.QueryEscape(q.Ext)
	}
	if q.Sort != "" {
		u += "&sort=" + url.QueryEscape(q.Sort)
	}
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}
	books := parseSearchRows(body)
	res := &model.BookSearchResult{Page: q.Page, Books: books, Total: len(books)}
	if res.Total == 0 && q.Page > 1 {
		res.Total = (q.Page - 1) * 10 // 粗略总数，联调时按分页栏校准
	}
	return res, nil
}

// Detail 书籍详情。
func (c *Client) Detail(ctx context.Context, bookID string) (*model.Book, error) {
	home, err := c.ensureLogin(ctx)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, fmt.Sprintf("%s/book/%s", home, bookID))
	if err != nil {
		return nil, fmt.Errorf("详情失败: %w", err)
	}
	b := parseDetail(body)
	if b == nil || b.Title == "" {
		return nil, fmt.Errorf("未解析到书籍 %s（选择器可能需要联调校准）", bookID)
	}
	b.ID = bookID
	return b, nil
}

// Download 打开下载流。
// TODO(联调)：免费账号下载入口为详情页「Download」按钮（含动态 token 的 /dlp/ 链接），
// 提供凭据后校准 reDLp 的匹配与 Referer/UA 要求。
var reDLp = regexp.MustCompile(`href="(/dlp/[^"]+)"`)

func (c *Client) Download(ctx context.Context, bookID string) (io.ReadCloser, int64, string, error) {
	home, err := c.ensureLogin(ctx)
	if err != nil {
		return nil, 0, "", err
	}
	body, err := c.get(ctx, fmt.Sprintf("%s/book/%s", home, bookID))
	if err != nil {
		return nil, 0, "", err
	}
	m := reDLp.FindStringSubmatch(body)
	if m == nil {
		return nil, 0, "", fmt.Errorf("未找到下载入口（额度用尽或页面结构变化）")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, home+m[1], nil)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", fmt.Sprintf("%s/book/%s", home, bookID))
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, "", fmt.Errorf("下载响应异常: HTTP %d", resp.StatusCode)
	}
	format := ""
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "epub") {
		format = "epub"
	} else if strings.Contains(ct, "pdf") {
		format = "pdf"
	}
	return resp.Body, resp.ContentLength, format, nil
}

const ua = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128 Safari/537.36"

// ensureLogin 惰性登录并返回个人域名首页。
// 已按真实站点（z-lib.ag，2026-09 联调）校准的流程：
//  1. GET  {base}/login   提取隐藏字段 _token（Laravel CSRF）
//  2. POST {base}/login    form: _token/email/password/remember
//  3. 判定：302 Location 指回 /login → 凭据错误；其他 Location（个人域名/首页）→ 成功
func (c *Client) ensureLogin(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn && c.home != "" {
		return c.home, nil
	}
	if c.opt.Email == "" || c.opt.Password == "" {
		return "", fmt.Errorf("Z-Library 凭据未配置（ZLIB_EMAIL/ZLIB_PASSWORD）")
	}

	// 1) 登录页拿 _token
	loginURL := strings.TrimRight(c.opt.BaseURL, "/") + "/login"
	body, err := c.get(ctx, loginURL)
	if err != nil {
		return "", fmt.Errorf("打开登录页失败: %w", err)
	}
	token := extractLoginToken(body)
	if token == "" {
		return "", fmt.Errorf("登录页解析失败：未找到 _token（站点结构可能变化）")
	}

	// 2) 提交登录表单（不自动跟随 302，读取 Location 判定结果）
	form := url.Values{
		"_token":   {token},
		"email":    {c.opt.Email},
		"password": {c.opt.Password},
		"remember": {"on"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", loginURL)
	req.Header.Set("Origin", strings.TrimRight(c.opt.BaseURL, "/"))
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	location := resp.Header.Get("Location")
	if location == "" || strings.HasSuffix(strings.Split("?", location)[0], "/login") {
		return "", fmt.Errorf("Z-Library 凭据错误（These credentials do not match our records）：请核对 ZLIB_EMAIL/ZLIB_PASSWORD")
	}

	// 3) 成功：Location 通常是个人域名（绝对 URL）或站内路径
	home := c.opt.BaseURL
	if strings.HasPrefix(location, "http") {
		home = location
	} else if loc, err := url.Parse(location); err == nil {
		home = strings.TrimRight(c.opt.BaseURL, "/") + loc.Path
	}
	c.home = strings.TrimRight(strings.Split(home, "?")[0], "/")
	c.loggedIn = true
	return c.home, nil
}

var reLoginToken = regexp.MustCompile(`name="_token"\s+value="([^"]+)"`)

func extractLoginToken(body string) string {
	if m := reLoginToken.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return ""
}

// get GET 请求并读取正文。
func (c *Client) get(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return string(b), err
}

// ---- HTML 解析（纯函数，便于离线单测） ----

func stripTags(s string) string {
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(s, " "))
}

func parseSearchRows(body string) []model.Book {
	var books []model.Book
	for _, row := range reRow.FindAllStringSubmatch(body, -1) {
		html := row[1]
		b := model.Book{}
		if m := reBookHref.FindStringSubmatch(html); m != nil {
			b.ID = m[1]
		}
		if m := reTitle.FindStringSubmatch(html); m != nil {
			b.Title = stripTags(m[1])
		}
		b.Authors = []string{}
		for _, a := range reAuthor.FindAllStringSubmatch(html, -1) {
			b.Authors = append(b.Authors, stripTags(a[1]))
		}
		if m := reCover.FindStringSubmatch(html); m != nil {
			b.CoverURL = m[1]
		}
		for _, kv := range reAttr.FindAllStringSubmatch(html, -1) {
			v := stripTags(kv[2])
			switch kv[1] {
			case "lang":
				b.Language = v
			case "ext":
				b.Extension = v
			case "size":
				b.SizeText = v
			case "year":
				b.Year = v
			}
		}
		if b.ID != "" && b.Title != "" {
			books = append(books, b)
		}
	}
	return books
}

func parseDetail(body string) *model.Book {
	b := &model.Book{}
	if m := reTitle.FindStringSubmatch(body); m != nil {
		b.Title = stripTags(m[1])
	}
	b.Authors = []string{}
	for _, a := range reAuthor.FindAllStringSubmatch(body, -1) {
		b.Authors = append(b.Authors, stripTags(a[1]))
	}
	if m := reDesc.FindStringSubmatch(body); m != nil {
		b.Description = stripTags(m[1])
	}
	for _, kv := range reAttr.FindAllStringSubmatch(body, -1) {
		v := stripTags(kv[2])
		switch kv[1] {
		case "lang":
			b.Language = v
		case "ext":
			b.Extension = v
		case "size":
			b.SizeText = v
		case "year":
			b.Year = v
		}
	}
	return b
}
