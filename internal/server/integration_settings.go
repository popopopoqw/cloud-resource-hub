package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"resource-center/internal/cfmon"
	"resource-center/internal/gmail"
	"resource-center/internal/pan123web"
	"resource-center/internal/r2"
	"resource-center/internal/respool"
	"resource-center/internal/zlib"
)

// GET /api/v1/settings/integrations
// 只返回非密钥值和“是否已配置”，不会把密码、Token、Secret 回传浏览器。
func (s *Server) handleIntegrationSettings(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	cfg := s.configSnapshot()
	accountID := cfg.CloudflareAccountID()
	forumHasLogin := false
	if pool := s.resourcePool(); pool != nil {
		forumHasLogin = pool.HasCookie()
	}
	writeOK(w, map[string]any{
		"pan123": map[string]any{
			"configured": cfg.Pan123Account != "" && cfg.Pan123Password != "",
			"account":    cfg.Pan123Account, "password_configured": cfg.Pan123Password != "",
		},
		"forum": map[string]any{
			"configured": cfg.ResPoolCookie != "" || (cfg.ResPoolAccount != "" && cfg.ResPoolPassword != "") || forumHasLogin,
			"has_login":  forumHasLogin,
			"account":    cfg.ResPoolAccount, "cookie_configured": cfg.ResPoolCookie != "", "password_configured": cfg.ResPoolPassword != "",
		},
		"gmail": map[string]any{
			"configured": cfg.GmailClientID != "" && cfg.GmailClientSecret != "" && cfg.GmailRefreshToken != "",
			"client_id":  cfg.GmailClientID, "email": cfg.GmailEmail,
			"client_secret_configured": cfg.GmailClientSecret != "", "refresh_token_configured": cfg.GmailRefreshToken != "",
		},
		"cloudflare": map[string]any{
			"configured": cfg.CFAPIToken != "" && accountID != "",
			"account_id": accountID, "api_token_configured": cfg.CFAPIToken != "",
		},
		"r2": map[string]any{
			"configured": accountID != "" && cfg.R2AccessKeyID != "" && cfg.R2SecretAccessKey != "" && cfg.R2Bucket != "",
			"bucket":     cfg.R2Bucket, "public_base": cfg.R2PublicBase,
			"access_key_configured": cfg.R2AccessKeyID != "", "secret_key_configured": cfg.R2SecretAccessKey != "",
		},
		"library": map[string]any{
			"configured": cfg.ZLibMode == "jump" || (cfg.ZLibEmail != "" && cfg.ZLibPassword != ""),
			"mode":       cfg.ZLibMode, "base_url": cfg.ZLibBaseURL, "email": cfg.ZLibEmail, "password_configured": cfg.ZLibPassword != "",
		},
	})
}

type integrationSettingsReq struct {
	Section string            `json:"section"`
	Values  map[string]string `json:"values"`
}

var integrationEnvKeys = map[string]map[string]string{
	"pan123":     {"account": "PAN123_ACCOUNT", "password": "PAN123_PASSWORD"},
	"forum":      {"cookie": "RESPOOL_COOKIE", "account": "RESPOOL_ACCOUNT", "password": "RESPOOL_PASSWORD"},
	"gmail":      {"client_id": "GMAIL_CLIENT_ID", "client_secret": "GMAIL_CLIENT_SECRET", "refresh_token": "GMAIL_REFRESH_TOKEN", "email": "GMAIL_EMAIL"},
	"cloudflare": {"api_token": "CF_API_TOKEN", "account_id": "CF_ACCOUNT_ID"},
	"r2":         {"access_key_id": "R2_ACCESS_KEY_ID", "secret_access_key": "R2_SECRET_ACCESS_KEY", "bucket": "R2_BUCKET", "public_base": "R2_PUBLIC_BASE"},
	"library":    {"mode": "ZLIB_MODE", "base_url": "ZLIB_BASE_URL", "email": "ZLIB_EMAIL", "password": "ZLIB_PASSWORD"},
}

// POST /api/v1/settings/r2/buckets
// 用当前或表单中尚未保存的凭据列举 Bucket，只向浏览器返回名称。
func (s *Server) handleListR2Buckets(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Values map[string]string `json:"values"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	for key, value := range req.Values {
		if _, ok := integrationEnvKeys["r2"][key]; !ok {
			writeErr(w, http.StatusBadRequest, "不支持的配置项: "+key)
			return
		}
		if strings.ContainsAny(value, "\r\n") {
			writeErr(w, http.StatusBadRequest, "配置值不能包含换行")
			return
		}
	}
	cfg := s.configSnapshot()
	accountID := cfg.CloudflareAccountID()
	if accountID == "" {
		writeErr(w, http.StatusBadRequest, "请先在 Cloudflare 配置中填写 Account ID")
		return
	}
	client := r2.New(accountID,
		candidate(req.Values, "access_key_id", cfg.R2AccessKeyID),
		candidate(req.Values, "secret_access_key", cfg.R2SecretAccessKey), "")
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	var restErr error
	if cfg.CFAPIToken != "" {
		if buckets, err := client.ListBucketsViaAPI(ctx, cfg.CFAPIToken); err == nil {
			writeOK(w, map[string]any{"buckets": buckets})
			return
		} else {
			restErr = err
		}
	}
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		if restErr != nil {
			writeErr(w, http.StatusBadGateway, "读取 Bucket 列表失败；Cloudflare API: "+restErr.Error()+"；R2 S3: "+err.Error())
		} else {
			writeErr(w, http.StatusBadGateway, err.Error())
		}
		return
	}
	writeOK(w, map[string]any{"buckets": buckets})
}

// POST /api/v1/settings/integrations/test
// 使用表单中的候选配置做只读连接测试；空字段沿用当前配置，不写入 .env。
func (s *Server) handleTestIntegrationSettings(w http.ResponseWriter, r *http.Request) {
	var req integrationSettingsReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	allowed, ok := integrationEnvKeys[req.Section]
	if !ok {
		writeErr(w, http.StatusBadRequest, "未知配置分组")
		return
	}
	for key, value := range req.Values {
		if _, ok := allowed[key]; !ok {
			writeErr(w, http.StatusBadRequest, "不支持的配置项: "+key)
			return
		}
		if strings.ContainsAny(value, "\r\n") {
			writeErr(w, http.StatusBadRequest, "配置值不能包含换行")
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	detail, err := s.testIntegration(ctx, req.Section, req.Values)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "测试失败: "+err.Error())
		return
	}
	writeOK(w, map[string]any{"passed": true, "detail": detail})
}

func candidate(values map[string]string, key, current string) string {
	if value := strings.TrimSpace(values[key]); value != "" {
		return value
	}
	return current
}

func (s *Server) testIntegration(ctx context.Context, section string, values map[string]string) (string, error) {
	cfg := s.configSnapshot()
	switch section {
	case "pan123":
		account := candidate(values, "account", cfg.Pan123Account)
		password := candidate(values, "password", cfg.Pan123Password)
		if account == "" || password == "" {
			return "", fmt.Errorf("请填写 123云盘账号和密码（Token 将自动获取与续期）")
		}
		client := pan123web.New("", account, password)
		result := make(chan error, 1)
		go func() {
			_, _, err := client.ListDirPage(0, 1, 1)
			result <- err
		}()
		select {
		case err := <-result:
			if err != nil {
				return "", err
			}
			return "123云盘连接与凭据验证通过", nil
		case <-ctx.Done():
			return "", fmt.Errorf("123云盘连接超时")
		}

	case "forum":
		cookie := candidate(values, "cookie", cfg.ResPoolCookie)
		account := candidate(values, "account", cfg.ResPoolAccount)
		password := candidate(values, "password", cfg.ResPoolPassword)
		if cookie == "" && (account == "" || password == "") {
			return "", fmt.Errorf("请同时填写论坛账号和密码")
		}
		fetcher := respool.New(cookie, account, password, cfg.DataDir, false)
		var passed []string
		for _, source := range respool.Sources {
			if err := fetcher.Probe(ctx, source.ID); err == nil {
				passed = append(passed, source.Name)
			}
		}
		if len(passed) == 0 {
			return "", fmt.Errorf("资源论坛均不可访问")
		}
		return "资源论坛可访问: " + strings.Join(passed, "、"), nil

	case "gmail":
		clientID := candidate(values, "client_id", cfg.GmailClientID)
		clientSecret := candidate(values, "client_secret", cfg.GmailClientSecret)
		refreshToken := candidate(values, "refresh_token", cfg.GmailRefreshToken)
		if clientID == "" || clientSecret == "" || refreshToken == "" {
			return "", fmt.Errorf("Client ID、Client Secret 和 Refresh Token 必须完整")
		}
		client := gmail.NewClient(clientID, clientSecret, refreshToken, candidate(values, "email", cfg.GmailEmail))
		if err := client.Probe(ctx); err != nil {
			return "", err
		}
		return "Gmail OAuth 授权验证通过", nil

	case "cloudflare":
		accountID := candidate(values, "account_id", cfg.CloudflareAccountID())
		apiToken := candidate(values, "api_token", cfg.CFAPIToken)
		if accountID == "" || apiToken == "" {
			return "", fmt.Errorf("Account ID 和 API Token 必须完整")
		}
		if err := cfmon.NewClient(apiToken, accountID).Probe(ctx); err != nil {
			return "", err
		}
		return "Cloudflare API 验证通过", nil

	case "r2":
		accountID := cfg.CloudflareAccountID()
		client := r2.New(
			accountID,
			candidate(values, "access_key_id", cfg.R2AccessKeyID),
			candidate(values, "secret_access_key", cfg.R2SecretAccessKey),
			candidate(values, "bucket", cfg.R2Bucket),
		)
		if err := client.Probe(ctx); err != nil {
			return "", err
		}
		return "R2 Bucket 连接与凭据验证通过", nil

	case "library":
		mode := candidate(values, "mode", cfg.ZLibMode)
		baseURL := candidate(values, "base_url", cfg.ZLibBaseURL)
		switch mode {
		case "jump":
			if baseURL == "" {
				return "", fmt.Errorf("请填写书库站点地址")
			}
			if err := zlib.NewJump(baseURL).Probe(ctx); err != nil {
				return "", err
			}
			return "书库入口可访问", nil
		case "direct":
			email := candidate(values, "email", cfg.ZLibEmail)
			password := candidate(values, "password", cfg.ZLibPassword)
			if baseURL == "" || email == "" || password == "" {
				return "", fmt.Errorf("direct 模式需要站点地址、邮箱和密码")
			}
			if err := zlib.New(zlib.Options{BaseURL: baseURL, Email: email, Password: password}).Probe(ctx); err != nil {
				return "", err
			}
			return "书库直连入口可访问", nil
		default:
			return "", fmt.Errorf("书库模式必须是 jump 或 direct")
		}
	}
	return "", fmt.Errorf("未知配置分组")
}

// POST /api/v1/settings/integrations
// 再次验证候选配置，通过后加密敏感值写入 SQLite，并热替换对应运行时客户端。
func (s *Server) handleSaveIntegrationSettings(w http.ResponseWriter, r *http.Request) {
	var req integrationSettingsReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	allowed, ok := integrationEnvKeys[req.Section]
	if !ok {
		writeErr(w, http.StatusBadRequest, "未知配置分组")
		return
	}
	updates := map[string]string{}
	for key, value := range req.Values {
		_, ok := allowed[key]
		if !ok {
			writeErr(w, http.StatusBadRequest, "不支持的配置项: "+key)
			return
		}
		value = strings.TrimSpace(value)
		if value == "" { // 密钥输入框留空时保留原值，避免误清除。
			continue
		}
		if strings.ContainsAny(value, "\r\n") {
			writeErr(w, http.StatusBadRequest, "配置值不能包含换行")
			return
		}
		updates[key] = value
	}
	if req.Section == "library" {
		if mode, ok := req.Values["mode"]; ok && mode != "jump" && mode != "direct" {
			writeErr(w, http.StatusBadRequest, "书库模式必须是 jump 或 direct")
			return
		}
	}
	if len(updates) == 0 {
		writeErr(w, http.StatusBadRequest, "没有需要保存的配置")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if _, err := s.testIntegration(ctx, req.Section, req.Values); err != nil {
		writeErr(w, http.StatusBadRequest, "保存前测试失败: "+err.Error())
		return
	}
	if err := s.persistIntegrationSettings(req.Section, updates); err != nil {
		writeErr(w, http.StatusInternalServerError, "保存 SQLite 配置失败: "+err.Error())
		return
	}
	s.applyIntegrationValues(req.Section, updates)
	writeOK(w, map[string]any{"saved": true, "applied": true, "restart_required": false})
}
