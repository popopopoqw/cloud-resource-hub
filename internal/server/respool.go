package server

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"resource-center/internal/respool"
)

const resourceMaxItems = 50

// GET /api/v1/respool/sources 资源源列表与登录态状态
func (s *Server) handleResPoolSources(w http.ResponseWriter, _ *http.Request) {
	type src struct {
		ID       string          `json:"id"`
		Name     string          `json:"name"`
		HasLogin bool            `json:"has_login"`
		Forums   []respool.Forum `json:"forums"`
	}
	out := []src{}
	pool := s.resourcePool()
	for _, v := range respool.Sources {
		forums := v.Forums
		if pool.HasCookie() {
			forums = pool.DiscoverForums(v.ID) // 登录态：自动发现全部版块
		}
		out = append(out, src{ID: v.ID, Name: v.Name, HasLogin: pool.HasCookie(), Forums: forums})
	}
	writeOK(w, out)
}

// GET /api/v1/respool/list?source=pan1me&page=1 帖子列表（游客态可抓）
func (s *Server) handleResPoolList(w http.ResponseWriter, r *http.Request) {
	srcID := r.URL.Query().Get("source")
	if srcID == "" {
		srcID = "pan1me"
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	result, err := s.resourcePool().List(srcID, r.URL.Query().Get("forum"), page)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	result = capThreadPage(result, resourceMaxItems)
	threads := result.Threads
	// 分类过滤（标题语义分类，影视内容混发于短剧版块）
	if cat := r.URL.Query().Get("category"); cat != "" && cat != "全部" {
		filtered := make([]respool.Thread, 0, len(threads))
		for _, t := range threads {
			if t.Category == cat {
				filtered = append(filtered, t)
			}
		}
		threads = filtered
	}
	writeOK(w, map[string]any{"list": threads, "source": srcID, "page": result.Page,
		"page_size": result.PageSize, "total": result.Total, "total_pages": result.TotalPages,
		"has_more":   result.Page < result.TotalPages,
		"categories": []string{"全部", "短剧", "电视剧", "电影", "动漫", "综艺", "纪录片", "其他"}})
}

// capThreadPage 将论坛的数千条历史数据限制在最新 max 条范围内。
// 论坛标准页为 20 条，因此 max=50 时最多暴露 3 页，末页只保留 10 条。
func capThreadPage(page respool.ThreadPage, max int) respool.ThreadPage {
	if max < 1 {
		page.Threads = nil
		page.Total = 0
		page.TotalPages = 0
		return page
	}
	pageSize := page.PageSize
	if pageSize < 1 {
		pageSize = len(page.Threads)
	}
	if pageSize < 1 {
		pageSize = max
	}
	if page.Total > max || page.Total == 0 {
		page.Total = max
	}
	page.TotalPages = (page.Total + pageSize - 1) / pageSize
	remaining := page.Total - (page.Page-1)*pageSize
	if remaining <= 0 {
		page.Threads = []respool.Thread{}
	} else if remaining < len(page.Threads) {
		page.Threads = page.Threads[:remaining]
	}
	return page
}

// GET /api/v1/respool/cover?source=pan1me&tid=123 首帖海报（无图返回空字符串）。
func (s *Server) handleResPoolCover(w http.ResponseWriter, r *http.Request) {
	tid := strings.TrimSpace(r.URL.Query().Get("tid"))
	if tid == "" {
		writeErr(w, http.StatusBadRequest, "tid 不能为空")
		return
	}
	for _, ch := range tid {
		if ch < '0' || ch > '9' {
			writeErr(w, http.StatusBadRequest, "tid 格式错误")
			return
		}
	}
	sourceID := r.URL.Query().Get("source")
	if sourceID == "" {
		sourceID = "pan1me"
	}
	coverURL, err := s.resourcePool().Cover(sourceID, tid)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, map[string]string{"cover_url": coverURL})
}

// POST /api/v1/respool/login 返回当前论坛登录状态；交互登录由应用内代理页面完成。
func (s *Server) handleResPoolLogin(w http.ResponseWriter, _ *http.Request) {
	if err := s.resourcePool().EnsureLogin(); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, map[string]bool{"has_login": true})
}

var forumHTMLURLAttr = regexp.MustCompile(`(?i)(href|src|action)=(['"])([^'"]+)['"]`)
var forumCSSURL = regexp.MustCompile(`(?i)url\(\s*(['"]?)([^)'"]+)['"]?\s*\)`)

// handleForumProxyAll 反向代理论坛页面、静态资源、验证码和登录请求。
// 代理 URL 保留远端路径结构，使相对 URL 和论坛原有 AJAX 登录逻辑可以原样工作。
func (s *Server) handleForumProxyAll(w http.ResponseWriter, r *http.Request) {
	pool := s.resourcePool()
	sourceID := r.PathValue("source")
	src, ok := respool.FindSource(sourceID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	remotePath := "/" + strings.TrimPrefix(r.PathValue("path"), "/")
	if remotePath == "/" {
		remotePath = "/user-login.htm"
	}
	targetURL := strings.TrimRight(src.Base, "/") + remotePath
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "代理请求构建失败")
		return
	}
	for _, name := range []string{"Content-Type", "X-Requested-With", "Accept", "Accept-Language"} {
		if value := r.Header.Get(name); value != "" {
			req.Header.Set(name, value)
		}
	}
	req.Header.Set("User-Agent", r.Header.Get("User-Agent"))
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 AppleWebKit/537.36 Chrome/128 Safari/537.36")
	}
	req.Header.Set("Origin", src.Base)
	req.Header.Set("Referer", strings.TrimRight(src.Base, "/")+remotePath)

	for _, cookie := range r.Cookies() {
		if isForumCookieName(cookie.Name) {
			pool.MergeProxySession(sourceID, cookie.Name+"="+cookie.Value)
		}
	}
	if session := pool.ProxySession(sourceID); session != "" {
		req.Header.Set("Cookie", session)
	}

	client := s.forumHTTP
	if client == nil {
		client = &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "论坛不可达: "+err.Error())
		return
	}
	defer resp.Body.Close()

	proxyPrefix := "/api/forum/proxy/" + sourceID
	wasLoggedIn := pool.HasCookie()
	for _, cookie := range resp.Cookies() {
		pool.MergeProxySession(sourceID, cookie.Name+"="+cookie.Value)
		cookie.Domain = ""
		cookie.Path = proxyPrefix
		cookie.Secure = r.TLS != nil
		cookie.SameSite = http.SameSiteLaxMode
		http.SetCookie(w, cookie)
	}

	loggedIn := pool.HasProxyLogin(sourceID)
	if loggedIn && !wasLoggedIn && pool.PersistProxySession(sourceID) {
		log.Printf("[respool] 论坛登录态验证通过并已持久化 source=%s", sourceID)
	}

	copyForumResponseHeaders(w.Header(), resp.Header)
	if location := resp.Header.Get("Location"); location != "" {
		w.Header().Set("Location", rewriteForumURL(location, src.Base+remotePath, proxyPrefix))
	}
	w.WriteHeader(resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(contentType, "text/html"):
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		page := rewriteForumHTML(string(data), src.Base+remotePath, proxyPrefix)
		if loggedIn {
			page = strings.Replace(page, "</body>", `<script>parent.postMessage({type:"forum-login-ok"}, window.location.origin);</script></body>`, 1)
		}
		_, _ = w.Write([]byte(page))
	case strings.Contains(contentType, "text/css"):
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_, _ = w.Write([]byte(rewriteForumCSS(string(data), src.Base+remotePath, proxyPrefix)))
	default:
		_, _ = io.Copy(w, resp.Body)
	}
}

func isForumCookieName(name string) bool {
	name = strings.ToLower(name)
	return strings.HasPrefix(name, "bbs_") || name == "cookie_test" || name == "cf_clearance" || name == "phpsessid"
}

func copyForumResponseHeaders(dst, src http.Header) {
	for _, name := range []string{"Content-Type", "Cache-Control", "Last-Modified", "ETag"} {
		if value := src.Get(name); value != "" {
			dst.Set(name, value)
		}
	}
}

func rewriteForumHTML(page, remotePageURL, proxyPrefix string) string {
	return forumHTMLURLAttr.ReplaceAllStringFunc(page, func(attr string) string {
		parts := forumHTMLURLAttr.FindStringSubmatch(attr)
		if len(parts) != 4 {
			return attr
		}
		return parts[1] + "=" + parts[2] + rewriteForumURL(parts[3], remotePageURL, proxyPrefix) + parts[2]
	})
}

func rewriteForumCSS(css, remotePageURL, proxyPrefix string) string {
	return forumCSSURL.ReplaceAllStringFunc(css, func(raw string) string {
		parts := forumCSSURL.FindStringSubmatch(raw)
		if len(parts) != 3 {
			return raw
		}
		return "url(" + parts[1] + rewriteForumURL(parts[2], remotePageURL, proxyPrefix) + parts[1] + ")"
	})
}

func rewriteForumURL(raw, remotePageURL, proxyPrefix string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "javascript:") ||
		strings.HasPrefix(trimmed, "data:") || strings.HasPrefix(trimmed, "mailto:") {
		return raw
	}
	base, err := url.Parse(remotePageURL)
	if err != nil {
		return raw
	}
	rel, err := url.Parse(trimmed)
	if err != nil {
		return raw
	}
	resolved := base.ResolveReference(rel)
	if resolved.Host != base.Host {
		return raw
	}
	result := proxyPrefix + resolved.EscapedPath()
	if resolved.RawQuery != "" {
		result += "?" + resolved.RawQuery
	}
	return result
}

// GET /api/v1/respool/thread// GET /api/v1/respool/thread?source=pan1me&tid=123 帖子正文（提取分享链接）
func (s *Server) handleResPoolThread(w http.ResponseWriter, r *http.Request) {
	tid := r.URL.Query().Get("tid")
	if tid == "" {
		writeErr(w, http.StatusBadRequest, "tid 不能为空")
		return
	}
	t, err := s.resourcePool().FetchThread(r.URL.Query().Get("source"), tid)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, t)
}
