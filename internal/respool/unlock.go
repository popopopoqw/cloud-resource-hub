package respool

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// ---- 论坛搜索与「探测 → 冷却 → 自动回帖解锁」状态机 ----
// 参照 Family Hub 蓝图模块 1/2 实现：曾回帖的帖子链接永久可见（秒提取），
// 未解锁则防封冷却后自动回帖一次，再行提取。

var (
	// 多域名分享链接（123pan / pan1.me 等变体，与蓝图正则同源）
	rePanLink = compileMust(`(?i)https?://[a-zA-Z0-9_.-]*(?:123[a-z0-9]*|pan1\.me|123pan)\.(?:com|cn|me)/(?:s|share|123pan)/([a-zA-Z0-9_-]+)`)
	rePanPwd  = compileMust(`(?i)(?:提取码|密码|访问码|pwd|code)[:：\s=]*([a-zA-Z0-9]{4,8})`)
	// 金币购买帖 / 失效帖特征
	reCoinPost = compileMust(`(?i)(金币|购买|付费|¥|¥)\s*(\d+)?`)
	reDeadPost = compileMust(`(?i)(链接失效|资源失效|已被删除|404)`)
)

// Search 论坛站内搜索（需登录态；游客态部分站也可搜）。
func (f *Fetcher) Search(sourceID, q string) ([]Thread, error) {
	src, ok := findSource(sourceID)
	if !ok {
		return nil, fmt.Errorf("未知资源源: %s", sourceID)
	}
	html, err := f.get(src.Base, "/?search-"+urlEscape(q)+".htm")
	if err != nil {
		return nil, fmt.Errorf("论坛搜索失败: %w", err)
	}
	if strings.Contains(html, "权限不足") {
		return nil, fmt.Errorf("搜索需登录态（先点「连接论坛」）")
	}
	seen := map[string]bool{}
	var threads []Thread
	for _, m := range reThreadLink.FindAllStringSubmatch(html, -1) {
		tid, title := m[1], strings.TrimSpace(stripTagsSimple(m[2]))
		if seen[tid] || title == "" {
			continue
		}
		seen[tid] = true
		threads = append(threads, Thread{
			ID: tid, Title: title, Category: classifyTitle(title),
			URL: strings.TrimRight(src.Base, "/") + "/thread-" + tid + ".htm",
		})
	}
	if len(threads) == 0 {
		return nil, fmt.Errorf("未搜到相关帖子（换关键词试试，如片名）")
	}
	return threads, nil
}

// ---- 防封冷却锁（全局单例：两次回帖间隔 ≥ 10s） ----

var (
	replyMu     sync.Mutex
	lastReplyAt time.Time
)

const replyCooldown = 10 * time.Second

// reserveReplySlot 原子检查并占用一次回帖窗口，避免并发点击同时通过冷却检查。
func reserveReplySlot() int {
	replyMu.Lock()
	defer replyMu.Unlock()
	elapsed := time.Since(lastReplyAt)
	if elapsed < replyCooldown {
		return int((replyCooldown - elapsed).Seconds()) + 1
	}
	lastReplyAt = time.Now()
	return 0
}

// defaultReplyText 自动回帖文案（可经 RESPOOL_REPLY_TEXT 配置）。
var defaultReplyText = "感谢分享，辛苦了！"

// SetReplyText 设置自动回帖文案。
func SetReplyText(t string) {
	if t = strings.TrimSpace(t); t != "" {
		defaultReplyText = t
	}
}

// UnlockAndFetch 蓝图模块2状态机：
//  1. 探测原帖（曾回帖 → 链接直接可见，秒级）
//  2. 未解锁 → 检查冷却（429 语义错误）→ 自动回帖 → 稍候重抓
//  3. 金币帖/失效帖识别
func (f *Fetcher) UnlockAndFetch(sourceID, tid string) (*Thread, error) {
	src, ok := findSource(sourceID)
	if !ok {
		return nil, fmt.Errorf("未知资源源: %s", sourceID)
	}
	if f.cookie() == "" {
		return nil, fmt.Errorf("论坛未登录：请点击「连接论坛」按钮")
	}

	build := func(html string) *Thread {
		t := &Thread{ID: tid, Fetched: true,
			URL: strings.TrimRight(src.Base, "/") + "/thread-" + tid + ".htm"}
		seen := map[string]bool{}
		for _, u := range rePanLink.FindAllString(html, -1) {
			if !seen[u] {
				seen[u] = true
				t.Shares = append(t.Shares, Share{URL: u})
			}
		}
		if m := rePanPwd.FindStringSubmatch(html); m != nil && len(t.Shares) > 0 {
			t.Shares[0].Code = m[1]
		}
		return t
	}

	// 1) 探测
	html, err := f.get(src.Base, "/thread-"+tid+".htm")
	if err != nil {
		return nil, err
	}
	if t := build(html); len(t.Shares) > 0 {
		t.AlreadyUnlocked = true
		return t, nil // 秒级路径（曾回帖或免回复帖）
	}
	// 检测"回复可见"（Xiuno 隐藏内容提示）
	needReply := strings.Contains(html, "请回复后再查看") || strings.Contains(html, "回复后查看") || strings.Contains(html, "特定内容")
	// 会话失效：直接报错（不再弹 Chrome，用户点「连接论坛」即可）
	if strings.Contains(html, "权限不足") || strings.Contains(html, "立即登录") {
		return nil, fmt.Errorf("论坛会话已失效：请重新点击「连接论坛」按钮")
	}
	// 2) 金币帖 / 失效帖识别
	if reCoinPost.MatchString(html) && strings.Contains(html, "购买") {
		return nil, fmt.Errorf("金币购买帖：系统无法代扣金币，请打开原帖手动购买")
	}
	if reDeadPost.MatchString(html) {
		return nil, fmt.Errorf("资源已失效或帖子被删")
	}

	// 2) 回复可见帖：冷却 + 自动回帖解锁（蓝图模块2）
	if !needReply {
		return nil, fmt.Errorf("帖子中未找到分享链接（可能为特殊格式，请打开原帖确认）")
	}

	if wait := reserveReplySlot(); wait > 0 {
		return nil, fmt.Errorf("论坛防封保护冷却中，请等待 %d 秒后重试", wait)
	}

	if err := f.reply(src.Base, tid); err != nil {
		return nil, fmt.Errorf("自动回帖失败: %w", err)
	}
	time.Sleep(1500 * time.Millisecond) // 等服务端落库

	// 3) 重抓提取
	html, err = f.get(src.Base, "/thread-"+tid+".htm")
	if err != nil {
		return nil, err
	}
	if t := build(html); len(t.Shares) > 0 {
		return t, nil
	}
	return nil, fmt.Errorf("回帖后仍未提取到分享链接（可能为金币帖或特殊隐藏）")
}

// reply 向帖子提交回复（Xiuno：POST ?post-create-{tid}-1.htm，Ajax 头）。
func (f *Fetcher) reply(base, tid string) error {
	body := strings.NewReader("message=" + urlEscape(defaultReplyText) + "&doctype=0")
	req, err := newPostRequest(base, "/?post-create-"+tid+"-1.htm", body)
	if err != nil {
		return err
	}
	if c := f.cookie(); c != "" {
		req.Header.Set("Cookie", c)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("读取回帖响应失败: %w", err)
	}
	if strings.Contains(string(payload), "立即登录") || strings.Contains(string(payload), "权限不足") {
		return fmt.Errorf("论坛会话已失效")
	}
	code, message, ok := forumReplyResult(payload)
	if !ok {
		return fmt.Errorf("论坛返回了无法识别的回帖响应")
	}
	if code != "0" {
		if message == "" {
			message = "论坛拒绝了回帖请求"
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

func forumReplyResult(payload []byte) (code, message string, ok bool) {
	var object map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&object); err == nil {
		code = fmt.Sprint(object["code"])
		message, _ = object["message"].(string)
		return code, message, code != "<nil>"
	}

	var array []any
	decoder = json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&array); err == nil && len(array) > 0 {
		code = fmt.Sprint(array[0])
		if len(array) > 1 {
			message, _ = array[1].(string)
		}
		return code, message, true
	}
	return "", "", false
}

func stripTagsSimple(s string) string {
	out := &strings.Builder{}
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}
