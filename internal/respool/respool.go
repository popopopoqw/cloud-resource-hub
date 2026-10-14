// Package respool 抓取 123云盘资源社区（pan1.me / 123panfx.com，Xiuno BBS 镜像站）
// 的帖子列表与正文中的分享链接，为「资源发现 → 一键转存」提供数据源。
//
// 访问模型：帖子列表游客可见（直接抓）；正文含分享链接但需登录态
// （RESPOOL_COOKIE 注入浏览器 cookie，绕过登录表单的极验验证码）。
package respool

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Thread 论坛帖子条目。
type Thread struct {
	ID              string  `json:"id"` // Xiuno tid
	Title           string  `json:"title"`
	URL             string  `json:"url"`              // 帖子详情页绝对地址
	Forum           string  `json:"forum"`            // 所属版块名
	Shares          []Share `json:"shares"`           // 正文中的分享链接（详情态填充）
	Fetched         bool    `json:"fetched"`          // 是否已抓过正文
	AlreadyUnlocked bool    `json:"already_unlocked"` // 曾回帖过，链接直接可见（秒级路径）
	Category        string  `json:"category"`         // 标题语义分类：短剧/电视剧/电影/动漫/综艺/纪录片/其他
}

// Share 提取到的网盘分享。
type Share struct {
	URL  string `json:"url"`  // 123pan 分享链接
	Code string `json:"code"` // 提取码（可为空）
}

// Forum 论坛版块。
type Forum struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Source 单个论坛源。Forums 为可用版块（登录态下自动发现全部；游客态为已知子集）。
type Source struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Base   string  `json:"base"`
	Forums []Forum `json:"forums"`
}

// Sources 内置源（两站互为镜像，独立配置便于择优）。
var Sources = []Source{
	{ID: "pan1me", Name: "pan1.me", Base: "https://pan1.me", Forums: []Forum{{"59", "短剧"}}},
	{ID: "pan123fx", Name: "123panfx", Base: "https://www.123panfx.com", Forums: []Forum{{"59", "短剧"}}},
}

// DiscoverForums 登录态下抓首页导航，自动发现全部版块（含游客隐藏的电影/电视剧/动漫/综艺等）。
// 返回补充后的版块列表；游客态返回已知子集。
func (f *Fetcher) DiscoverForums(sourceID string) []Forum {
	src, ok := findSource(sourceID)
	if !ok {
		return nil
	}
	html, err := f.get(src.Base, "/")
	if err != nil {
		return src.Forums
	}
	seen := map[string]bool{}
	var out []Forum
	for _, m := range reForumNav.FindAllStringSubmatch(html, -1) {
		fid, name := m[1], strings.TrimSpace(m[2])
		if fid == "" || name == "" || seen[fid] || len(name) > 16 {
			continue
		}
		seen[fid] = true
		out = append(out, Forum{ID: fid, Name: name})
	}
	if len(out) > len(src.Forums) {
		src.Forums = sortForumsByPriority(out) // 排序：电视剧/电影优先
		return src.Forums
	}
	return sortForumsByPriority(src.Forums)
}

// sortForumsByPriority 版块排序：电视剧/电影/动漫/综艺优先于短剧/公告（默认展示更有价值的版块）。
func sortForumsByPriority(forums []Forum) []Forum {
	priority := func(name string) int {
		switch {
		case strings.Contains(name, "电视剧"):
			return 0
		case strings.Contains(name, "电影"):
			return 1
		case strings.Contains(name, "动漫"):
			return 2
		case strings.Contains(name, "综艺"):
			return 3
		case strings.Contains(name, "综合"):
			return 4
		case strings.Contains(name, "短剧"):
			return 5
		default:
			return 6 // 公告等排最后
		}
	}
	out := make([]Forum, len(forums))
	copy(out, forums)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if priority(out[j].Name) < priority(out[i].Name) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

var reForumNav = regexp.MustCompile(`href="(?:\?|/|https?://[^"]*?/)?forum-(\d+)(?:-\d+)?\.htm"[^>]*>\s*([^<\s][^<]{0,14}?)\s*</a>`)

// Fetcher 论坛抓取器：游客态列表 + cookie 态正文，列表结果短缓存。
// 登录态策略：RESPOOL_COOKIE 直接注入 > 持久化会话 > chromedp 自动登录。
type Fetcher struct {
	cookieRaw string
	client    *http.Client
	sess      *sessionManager
	headed    bool // 自动登录是否弹出浏览器窗口（人工过滑块用）

	mu            sync.Mutex
	listCch       map[string]cachedList
	coverCch      map[string]cachedCover
	proxySessions map[string]map[string]string // source ID -> cookie name -> value
	loginMu       sync.Mutex
}

type cachedList struct {
	page     ThreadPage
	cachedAt time.Time
}

type cachedCover struct {
	url      string
	cachedAt time.Time
}

// ThreadPage 论坛版块的一页主帖子及真实分页信息。
type ThreadPage struct {
	Threads    []Thread
	Page       int
	PageSize   int
	Total      int
	TotalPages int
}

const (
	listTTL = 5 * time.Minute
	ua      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128 Safari/537.36"
)

// New 创建抓取器。
// cookieRaw：手动注入（最高优先级）；account/password：自动登录凭据；
// dataDir：会话持久化目录；headed：自动登录时是否弹真窗口。
func New(cookieRaw, account, password, dataDir string, headed bool) *Fetcher {
	jar, _ := cookiejar.New(nil)
	return &Fetcher{
		cookieRaw:     strings.TrimSpace(cookieRaw),
		client:        &http.Client{Timeout: 30 * time.Second, Jar: jar},
		sess:          newSessionManager(Sources[0].Base, account, password, dataDir, headed),
		headed:        headed,
		listCch:       map[string]cachedList{},
		coverCch:      map[string]cachedCover{},
		proxySessions: map[string]map[string]string{},
	}
}

// ---- 首帖海报（资源广场异步加载） ----

var (
	reFirstPostMarker = regexp.MustCompile(`(?i)\bisfirst\s*=\s*["']1["']`)
	reImageTag        = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	reImageAttr       = regexp.MustCompile(`(?is)\b(data-original|data-src|src)\s*=\s*["']([^"']+)["']`)
)

// Cover 抓取首帖正文中的第一张有效图片。结果（含无图）缓存一小时，
// 避免资源广场反复刷新时频繁请求论坛详情页。
func (f *Fetcher) Cover(sourceID, tid string) (string, error) {
	src, ok := findSource(sourceID)
	if !ok {
		return "", fmt.Errorf("未知资源源: %s", sourceID)
	}
	key := sourceID + ":" + tid
	f.mu.Lock()
	if cached, ok := f.coverCch[key]; ok && time.Since(cached.cachedAt) < time.Hour {
		f.mu.Unlock()
		return cached.url, nil
	}
	f.mu.Unlock()

	pageHTML, err := f.get(src.Base, "/thread-"+tid+".htm")
	if err != nil {
		return "", err
	}
	cover := extractCoverURL(pageHTML, src.Base)
	f.mu.Lock()
	f.coverCch[key] = cachedCover{url: cover, cachedAt: time.Now()}
	f.mu.Unlock()
	return cover, nil
}

func extractCoverURL(pageHTML, baseURL string) string {
	marker := reFirstPostMarker.FindStringIndex(pageHTML)
	if marker == nil {
		return ""
	}
	start := strings.LastIndex(pageHTML[:marker[0]], "<div")
	if start < 0 {
		start = marker[1]
	}
	section := pageHTML[start:]
	if end := strings.Index(section, `<div class="haya-post-like`); end >= 0 {
		section = section[:end]
	} else if len(section) > 128<<10 {
		section = section[:128<<10]
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	for _, tag := range reImageTag.FindAllString(section, -1) {
		attrs := map[string]string{}
		for _, match := range reImageAttr.FindAllStringSubmatch(tag, -1) {
			attrs[strings.ToLower(match[1])] = html.UnescapeString(strings.TrimSpace(match[2]))
		}
		candidate := attrs["data-original"]
		if candidate == "" {
			candidate = attrs["data-src"]
		}
		if candidate == "" {
			candidate = attrs["src"]
		}
		lower := strings.ToLower(candidate)
		if candidate == "" || strings.HasPrefix(lower, "data:") ||
			containsAny(lower, "logo", "avatar", "emoji", "smile", "广告", "/view/img/", "/upload/forum/") {
			continue
		}
		resolved, err := base.Parse(candidate)
		if err != nil || (resolved.Scheme != "http" && resolved.Scheme != "https") {
			continue
		}
		return resolved.String()
	}
	return ""
}

// cookie 返回当前生效的登录 cookie：手动注入 > 持久化会话。
// 持久化会话超过 25 天未刷新时后台尝试重登（不阻塞本次请求）。
func (f *Fetcher) cookie() string {
	f.mu.Lock()
	raw := f.cookieRaw
	f.mu.Unlock()
	if validForumCookie(raw) {
		return raw
	}
	c, at, err := f.sess.load()
	if err != nil || !validForumCookie(c) {
		return ""
	}
	if time.Since(at) > 25*24*time.Hour {
		go f.relogin()
	}
	return c
}

// HasCookie 是否具备登录态（含持久化会话）。
func (f *Fetcher) HasCookie() bool { return validForumCookie(f.cookie()) }

// Probe 验证指定论坛源的网络可达性；4xx 也表示目标站点已响应。
func (f *Fetcher) Probe(ctx context.Context, sourceID string) error {
	src, ok := findSource(sourceID)
	if !ok {
		return fmt.Errorf("未知资源源: %s", sourceID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, src.Base+"/", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", ua)
	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("网络请求失败: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// relogin 已废弃：Chrome 自动登录被反代登录替代。
func (f *Fetcher) relogin() bool {
	return false // 不再启动 Chrome
}

// EnsureLogin 已废弃：反代登录替代（点「连接论坛」按钮即可）。
func (f *Fetcher) EnsureLogin() error {
	if f.cookie() != "" {
		return nil
	}
	return fmt.Errorf("论坛未登录：请点击「连接论坛」按钮")
}

func (f *Fetcher) get(base, path string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+path, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	if c := f.cookie(); c != "" {
		req.Header.Set("Cookie", c)
	}
	req.Header.Set("Referer", base+"/")
	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("网络请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return string(b), err
}

// ---- 列表（游客可见） ----

var (
	reThreadLink = regexp.MustCompile("<a[^>]+href=\"(?:\\?|/|https?://[^\" ]*?/)?thread-(\\d+)\\.htm[^\"]*\"[^>]*>(.*?)</a>")
	reForumLink  = regexp.MustCompile(`<a[^>]+href="(?:https?://[^"]*?/)?forum-(\d+)\.htm"[^>]*>([^<]+)</a>`)
	reTopicTotal = regexp.MustCompile(`主题\s*:\s*<b>([\d,]+)</b>`)
	reForumHref  = regexp.MustCompile(`href=["'][^"']*forum-(\d+)\.htm["']`)
	reForumTotal = regexp.MustCompile(`主题数\s*[：:]\s*(?:<[^>]+>\s*)*([\d,]+)`)
	reForumPage  = regexp.MustCompile(`forum-(\d+)-(\d+)\.htm`)
)

// List 抓取指定版块的帖子列表，带缓存。forumID 为空时用源的第一个版块。
func (f *Fetcher) List(sourceID string, forumID string, page int) (ThreadPage, error) {
	src, ok := findSource(sourceID)
	if !ok {
		return ThreadPage{}, fmt.Errorf("未知资源源: %s", sourceID)
	}
	if forumID == "" {
		if f.HasCookie() {
			forums := f.DiscoverForums(sourceID) // 登录态：用最新版块列表
			if len(forums) > 0 {
				forumID = forums[0].ID
			}
		}
		if forumID == "" && len(src.Forums) > 0 {
			forumID = src.Forums[0].ID
		}
	}
	key := fmt.Sprintf("%s:%s:%d", sourceID, forumID, page)
	f.mu.Lock()
	if c, ok := f.listCch[key]; ok && time.Since(c.cachedAt) < listTTL {
		f.mu.Unlock()
		return c.page, nil
	}
	f.mu.Unlock()

	// 版块列表页（数据干净：纯资源帖、最新排序、标准分页），而非首页聚合区
	html, err := f.get(src.Base, fmt.Sprintf("/?forum-%s-%d.htm", forumID, page))
	if err != nil {
		return ThreadPage{}, err
	}
	result, err := parseThreadPage(html, src, forumID, page)
	if err != nil {
		return ThreadPage{}, err
	}
	f.mu.Lock()
	f.listCch[key] = cachedList{page: result, cachedAt: time.Now()}
	f.mu.Unlock()
	return result, nil
}

func parseThreadPage(pageHTML string, src Source, forumID string, page int) (ThreadPage, error) {
	mainHTML := pageHTML
	if start := strings.Index(mainHTML, `<ul class="list-unstyled threadlist`); start >= 0 {
		mainHTML = mainHTML[start:]
		if end := strings.Index(mainHTML, `<ul class="pagination`); end >= 0 {
			mainHTML = mainHTML[:end]
		}
	}

	seen := map[string]bool{}
	var threads []Thread
	mainCount := 0
	for _, m := range reThreadLink.FindAllStringSubmatch(mainHTML, -1) {
		tid, title := m[1], cleanTitle(m[2])
		if seen[tid] || title == "" {
			continue
		}
		seen[tid] = true
		mainCount++
		if isNoiseThread(title) {
			continue
		}
		threads = append(threads, Thread{
			ID: tid, Title: title, Category: classifyTitle(title),
			URL: strings.TrimRight(src.Base, "/") + "/thread-" + tid + ".htm",
		})
	}
	if len(threads) == 0 {
		return ThreadPage{}, fmt.Errorf("未解析到主帖子列表（页面结构可能变化）")
	}

	total := forumTopicTotal(pageHTML, forumID)
	totalPages := page
	for _, match := range reForumPage.FindAllStringSubmatch(pageHTML, -1) {
		if match[1] != forumID {
			continue
		}
		candidate, _ := strconv.Atoi(match[2])
		if candidate > totalPages {
			totalPages = candidate
		}
	}
	if total == 0 {
		total = (totalPages-1)*mainCount + len(threads)
	}
	return ThreadPage{
		Threads: threads, Page: page, PageSize: mainCount,
		Total: total, TotalPages: totalPages,
	}, nil
}

// forumTopicTotal 只读取当前版块卡片内的主题数。页面同时列出多个版块，
// 直接匹配第一个“主题”会误把其他版块（例如反馈区）的数量当作当前总数。
func forumTopicTotal(pageHTML, forumID string) int {
	for _, match := range reForumHref.FindAllStringSubmatchIndex(pageHTML, -1) {
		if pageHTML[match[2]:match[3]] != forumID {
			continue
		}
		endOffset := strings.Index(pageHTML[match[1]:], "</a>")
		if endOffset < 0 {
			continue
		}
		anchorHTML := pageHTML[match[0] : match[1]+endOffset+len("</a>")]
		if topic := reTopicTotal.FindStringSubmatch(anchorHTML); topic != nil {
			total, _ := strconv.Atoi(strings.ReplaceAll(topic[1], ",", ""))
			return total
		}
	}

	// 部分主题会在当前版块简介中使用“主题数：<span>...</span>”。
	if match := reForumTotal.FindStringSubmatch(pageHTML); match != nil {
		total, _ := strconv.Atoi(strings.ReplaceAll(match[1], ",", ""))
		return total
	}
	return 0
}

// ---- 详情（需登录态） ----

var (
	reShareLink = regexp.MustCompile(`https?://(?:www\.)?123(?:pan|yunpan)\.(?:com|cn)/s/[A-Za-z0-9_-]+`)
	reCode      = regexp.MustCompile(`(?:提取码|提取碼|密码|密碼|访问码)[^\w]{0,6}([A-Za-z0-9]{4,6})`)
)

// FetchThread 抓帖子正文（委托给 UnlockAndFetch：探测/回帖解锁/提取）。
func (f *Fetcher) FetchThread(sourceID, tid string) (*Thread, error) {
	return f.UnlockAndFetch(sourceID, tid)
}

// fetchThreadRaw 原始抓取（UnlockAndFetch 内部使用，不再导出）。
func (f *Fetcher) fetchThreadRaw(sourceID, tid string) (*Thread, string, error) {
	src, ok := findSource(sourceID)
	if !ok {
		return nil, "", fmt.Errorf("未知资源源: %s", sourceID)
	}
	if f.cookie() == "" {
		return nil, "", fmt.Errorf("论坛未登录：请点击「连接论坛」")
	}
	html, err := f.get(src.Base, "/thread-"+tid+".htm")
	if err != nil {
		return nil, "", err
	}
	if strings.Contains(html, "权限不足") || strings.Contains(html, "立即登录") {
		return nil, html, fmt.Errorf("论坛会话已失效：请重新点击「连接论坛」")
	}
	t := &Thread{ID: tid, URL: strings.TrimRight(src.Base, "/") + "/thread-" + tid + ".htm", Fetched: true}
	if m := regexp.MustCompile(`<title>(.*?)</title>`).FindStringSubmatch(html); m != nil {
		t.Title = strings.TrimSpace(strings.Split(m[1], "-")[0])
	}
	seen := map[string]bool{}
	for _, u := range reShareLink.FindAllString(html, -1) {
		if !seen[u] {
			seen[u] = true
			t.Shares = append(t.Shares, Share{URL: u})
		}
	}
	// 提取码（全文第一个，通常对应第一个链接）
	if m := reCode.FindStringSubmatch(html); m != nil {
		if len(t.Shares) > 0 {
			t.Shares[0].Code = m[1]
		}
	}
	if len(t.Shares) == 0 {
		return t, html, nil // 返回 html 供上层判断是否需要回帖
	}
	return t, html, nil
}

var reTitleSeason = regexp.MustCompile(`第[一二三四五六七八九十\d]季|S\d{1,2}[E/]|全\d+集`)

// classifyTitle 按标题语义分类（pan1.me 影视内容混发于短剧版块，靠标题区分）。
func classifyTitle(title string) string {
	switch {
	case containsAny(title, "短剧", "Ai漫剧", "AI漫剧", "微短剧", "横屏短剧", "竖屏短剧"):
		return "短剧"
	case containsAny(title, "电视剧", "美剧", "韩剧", "日剧", "国产剧", "英剧", "Netflix", "HBO", "迪士尼", "剧集"), reTitleSeason.MatchString(title):
		return "电视剧"
	case containsAny(title, "电影", "剧场版", "影院", "蓝光原盘", "REMUX", "BDRip", "WEB-DL"):
		return "电影"
	case containsAny(title, "动漫", "番剧", "动画", "漫画改", "国漫", "日漫"):
		return "动漫"
	case containsAny(title, "综艺", "真人秀", "脱口秀"):
		return "综艺"
	case containsAny(title, "纪录片", "纪实"):
		return "纪录片"
	default:
		return "其他"
	}
}

func containsAny(s string, kws ...string) bool {
	for _, kw := range kws {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// isNoiseThread 公告/版规/导航类非资源帖过滤。
func isNoiseThread(title string) bool {
	for _, kw := range []string{"公告", "版规", "必看", "频道", "群组", "安卓版本", "社区", "举报", "教程"} {
		if strings.Contains(title, kw) {
			return true
		}
	}
	return false
}

// cleanTitle 标题清理：去残留标签、HTML entity 反转义、压缩空白。
func cleanTitle(raw string) string {
	t := stripTagsSimple(raw)
	t = html.UnescapeString(t)
	t = strings.Join(strings.Fields(t), " ")
	return strings.TrimLeft(t, "*★☆· ")
}

// ProxySession 返回指定论坛源当前的代理会话 Cookie。
func (f *Fetcher) ProxySession(sourceID string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return renderCookieMap(f.proxySessions[sourceID])
}

// MergeProxySession 合并一个 Set-Cookie 或 Cookie name=value 片段。
// deleted/空值会删除对应 Cookie，避免把退出登录响应误判为有效会话。
func (f *Fetcher) MergeProxySession(sourceID, pair string) {
	name, value, ok := parseCookiePair(pair)
	if !ok {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.proxySessions[sourceID] == nil {
		f.proxySessions[sourceID] = map[string]string{}
	}
	if value == "" || strings.EqualFold(value, "deleted") {
		delete(f.proxySessions[sourceID], name)
		return
	}
	f.proxySessions[sourceID][name] = value
}

// FindSource 导出版块查找（反代登录 handler 使用）。
func FindSource(id string) (Source, bool) { return findSource(id) }

// ProxyCookie 返回当前登录 cookie（反代页面带 cookie 渲染）。
func (f *Fetcher) ProxyCookie() string { return f.cookie() }

// SaveProxyCookie 保存反代登录拦截到的 Set-Cookie 并持久化。
// 每个 Set-Cookie 头形如 "name=value; Path=/; Domain=x; HttpOnly"，
// 只取第一个 name=value 对（其余是属性，不进请求头）。
func (f *Fetcher) SaveProxyCookie(setCookies []string) {
	cookies := map[string]string{}
	for _, sc := range setCookies {
		for _, part := range strings.Split(sc, ";") {
			name, value, ok := parseCookiePair(part)
			if !ok || isCookieAttribute(name) {
				continue
			}
			if value == "" || strings.EqualFold(value, "deleted") {
				delete(cookies, name)
				continue
			}
			cookies[name] = value
		}
	}
	cookie := renderCookieMap(cookies)
	if validForumCookie(cookie) {
		f.mu.Lock()
		f.cookieRaw = cookie
		f.mu.Unlock()
		_ = f.sess.save(cookie)
		fmt.Printf("[respool] 反代 cookie 已保存（%d 字段）\n", len(cookies))
	}
}

// PersistProxySession 验证并持久化指定论坛源的代理会话。
func (f *Fetcher) PersistProxySession(sourceID string) bool {
	cookie := f.ProxySession(sourceID)
	if !validForumCookie(cookie) {
		return false
	}
	f.mu.Lock()
	f.cookieRaw = cookie
	f.mu.Unlock()
	if err := f.sess.save(cookie); err != nil {
		return false
	}
	return true
}

// HasProxyLogin 判断指定论坛源的代理会话是否已有有效登录 token。
func (f *Fetcher) HasProxyLogin(sourceID string) bool {
	return validForumCookie(f.ProxySession(sourceID))
}

func validForumCookie(cookie string) bool {
	for _, part := range strings.Split(cookie, ";") {
		name, value, ok := parseCookiePair(part)
		if ok && name == "bbs_token" && value != "" && !strings.EqualFold(value, "deleted") {
			return true
		}
	}
	return false
}

func parseCookiePair(pair string) (string, string, bool) {
	pair = strings.TrimSpace(strings.SplitN(pair, ";", 2)[0])
	i := strings.IndexByte(pair, '=')
	if i <= 0 {
		return "", "", false
	}
	name := strings.TrimSpace(pair[:i])
	value := strings.TrimSpace(pair[i+1:])
	return name, value, name != ""
}

func isCookieAttribute(name string) bool {
	switch strings.ToLower(name) {
	case "path", "domain", "expires", "max-age", "samesite", "secure", "httponly", "partitioned", "priority":
		return true
	default:
		return false
	}
}

func renderCookieMap(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}
	slices.Sort(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+cookies[name])
	}
	return strings.Join(parts, "; ")
}

func findSource(id string) (Source, bool) {
	for _, s := range Sources {
		if s.ID == id {
			return s, true
		}
	}
	return Source{}, false
}

// ---- unlock.go 依赖的小工具 ----

func compileMust(expr string) *regexp.Regexp { return regexp.MustCompile(expr) }

func newPostRequest(base, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", base+"/")
	return req, nil
}

func urlEscape(s string) string { return url.QueryEscape(strings.TrimSpace(s)) }
