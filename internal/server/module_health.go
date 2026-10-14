package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"resource-center/internal/model"
	"resource-center/internal/respool"
)

// 探测用的浏览器 UA：部分站点对无 UA/爬虫 UA 直接拦截。
const probeUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128 Safari/537.36"

// probeReachable 纯连通性探测：只验证站点网络可达，不依赖任何账号凭据。
// HTTP <500 即视为可达（401/403/404 都说明网络层与服务本身正常）；
// HTTP 失败时降级为 TCP 连通测试——部分站点（如 Z-Library DiamWall）按 TLS
// 指纹拦截非浏览器客户端，但 DNS 解析与 TCP 连接成功足以证明站点存活。
// 凭据有效性由设置中心保存时的「测试连接」负责，二者职责分离。
func probeReachable(ctx context.Context, name, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", probeUA)
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1))
		if resp.StatusCode < 500 {
			return fmt.Sprintf("%s 可达（HTTP %d）", name, resp.StatusCode), nil
		}
		return "", fmt.Errorf("%s 异常（HTTP %d）", name, resp.StatusCode)
	}
	u, perr := url.Parse(rawURL)
	if perr != nil {
		return "", fmt.Errorf("%s 不可达: %w", name, err)
	}
	port := "443"
	if u.Scheme == "http" {
		port = "80"
	}
	var d net.Dialer
	conn, derr := d.DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	if derr != nil {
		return "", fmt.Errorf("%s 不可达: %w", name, err)
	}
	conn.Close()
	return fmt.Sprintf("%s 可达（TCP 连通）", name), nil
}

type moduleProbeResult struct {
	id     string
	detail string
	err    error
}

const moduleHealthInterval = 30 * time.Minute

var moduleHealthIDs = []string{
	"transfer", "library", "drive", "mailbox", "monitor",
	"settings", "subscribe", "share", "offline",
}

// GET /api/v1/module-health
// 只读取数据库中的最近一次探测结果，不在页面请求中访问任何外部服务。
func (s *Server) handleModuleHealth(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListModuleHealth()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "读取服务状态缓存失败")
		return
	}
	statuses := make(map[string]model.ModuleHealth, len(moduleHealthIDs))
	for _, id := range moduleHealthIDs {
		statuses[id] = model.ModuleHealth{ModuleID: id, Label: "待检测", Detail: "等待服务端定时检测"}
	}
	for _, item := range items {
		statuses[item.ModuleID] = item
	}
	writeOK(w, statuses)
}

// StartBackgroundTasks 启动服务端后台任务。健康检测启动时立即执行一次，
// 此后固定每 30 分钟刷新数据库缓存。
func (s *Server) StartBackgroundTasks(ctx context.Context) {
	go s.moduleHealthLoop(ctx)
}

func (s *Server) moduleHealthLoop(ctx context.Context) {
	s.refreshModuleHealth(ctx)
	ticker := time.NewTicker(moduleHealthInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.refreshModuleHealth(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// refreshModuleHealth 并发探测概览卡片背后的真实服务并持久化结果；
// 单轮最多等待 8 秒，互斥锁避免手动/定时触发发生重叠。
func (s *Server) refreshModuleHealth(parent context.Context) {
	s.moduleHealthMu.Lock()
	defer s.moduleHealthMu.Unlock()

	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()

	statuses := map[string]model.ModuleHealth{
		"settings":  {ModuleID: "settings", Available: true, Label: "可用", Detail: "本地配置服务正常"},
		"subscribe": {ModuleID: "subscribe", Label: "异常", Detail: "资源订阅服务尚未接入"},
		"share":     {ModuleID: "share", Label: "异常", Detail: "分享管理服务尚未接入"},
		"offline":   {ModuleID: "offline", Label: "异常", Detail: "离线下载服务尚未接入"},
	}

	probes := []struct {
		id string
		fn func(context.Context) (string, error)
	}{
		{"transfer", s.probeResourceSites},
		// 以下模块只探测外部站点连通性，与是否配置账号无关
		{"library", func(ctx context.Context) (string, error) {
			base := s.configSnapshot().ZLibBaseURL
			if base == "" {
				base = "https://z-lib.sk"
			}
			return probeReachable(ctx, "Z-Library", base)
		}},
		{"drive", func(ctx context.Context) (string, error) {
			return probeReachable(ctx, "123云盘", "https://www.123pan.com")
		}},
		{"mailbox", func(ctx context.Context) (string, error) {
			return probeReachable(ctx, "Gmail 服务", "https://gmail.googleapis.com")
		}},
		{"monitor", func(ctx context.Context) (string, error) {
			return probeReachable(ctx, "Cloudflare API", "https://api.cloudflare.com")
		}},
	}

	results := make(chan moduleProbeResult, len(probes))
	pending := make(map[string]bool, len(probes))
	for _, probe := range probes {
		pending[probe.id] = true
		go func(id string, fn func(context.Context) (string, error)) {
			detail, err := fn(ctx)
			results <- moduleProbeResult{id: id, detail: detail, err: err}
		}(probe.id, probe.fn)
	}

	for len(pending) > 0 {
		select {
		case result := <-results:
			delete(pending, result.id)
			if result.err != nil {
				statuses[result.id] = model.ModuleHealth{ModuleID: result.id, Label: "异常", Detail: result.err.Error()}
			} else {
				statuses[result.id] = model.ModuleHealth{ModuleID: result.id, Available: true, Label: "可用", Detail: result.detail}
			}
		case <-ctx.Done():
			for id := range pending {
				statuses[id] = model.ModuleHealth{ModuleID: id, Label: "异常", Detail: "服务探测超时"}
			}
			pending = map[string]bool{}
		}
	}

	checkedAt := time.Now()
	items := make([]model.ModuleHealth, 0, len(statuses))
	for _, id := range moduleHealthIDs {
		item, ok := statuses[id]
		if !ok {
			item = model.ModuleHealth{ModuleID: id, Label: "异常", Detail: "未返回探测结果"}
		}
		item.CheckedAt = checkedAt
		items = append(items, item)
	}
	if err := s.store.SaveModuleHealth(items); err != nil {
		log.Printf("保存模块健康状态失败: %v", err)
		return
	}
	log.Printf("模块健康状态已更新，下次检测时间约为 %s", checkedAt.Add(moduleHealthInterval).Format("2006-01-02 15:04:05"))
}

// probeResourceSites 探测全部论坛源；任一源可达即可支撑资源发现功能。
func (s *Server) probeResourceSites(ctx context.Context) (string, error) {
	type result struct {
		name string
		err  error
	}
	results := make(chan result, len(respool.Sources))
	pool := s.resourcePool()
	for _, source := range respool.Sources {
		go func(id, name string) {
			results <- result{name: name, err: pool.Probe(ctx, id)}
		}(source.ID, source.Name)
	}

	var available, failed []string
	for range respool.Sources {
		select {
		case result := <-results:
			if result.err == nil {
				available = append(available, result.name)
			} else {
				failed = append(failed, result.name)
			}
		case <-ctx.Done():
			return "", fmt.Errorf("资源网站探测超时")
		}
	}
	if len(available) == 0 {
		return "", fmt.Errorf("资源网站均不可访问: %s", strings.Join(failed, "、"))
	}
	detail := "可访问: " + strings.Join(available, "、")
	if len(failed) > 0 {
		detail += "；异常: " + strings.Join(failed, "、")
	}
	return detail, nil
}
