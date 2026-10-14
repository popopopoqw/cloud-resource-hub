package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"resource-center/internal/gmail"
)

const gmailOAuthTTL = 10 * time.Minute

type gmailOAuthPending struct {
	ClientID     string
	ClientSecret string
	Email        string
	RedirectURI  string
	UserID       string
	ExpiresAt    time.Time
}

type gmailOAuthStartReq struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Email        string `json:"email"`
}

// POST /api/v1/settings/gmail/oauth/start
// 只返回 Google 授权地址；Client Secret 仅暂存在服务端内存中。
func (s *Server) handleGmailOAuthStart(w http.ResponseWriter, r *http.Request) {
	var req gmailOAuthStartReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg := s.configSnapshot()
	clientID := candidate(map[string]string{"client_id": req.ClientID}, "client_id", cfg.GmailClientID)
	clientSecret := candidate(map[string]string{"client_secret": req.ClientSecret}, "client_secret", cfg.GmailClientSecret)
	email := candidate(map[string]string{"email": req.Email}, "email", cfg.GmailEmail)
	if clientID == "" || clientSecret == "" {
		writeErr(w, http.StatusBadRequest, "请先填写 Client ID 和 Client Secret")
		return
	}

	state := newToken()
	redirect := requestOrigin(cfg.BaseURL, r) + "/api/v1/settings/gmail/oauth/callback"
	now := time.Now()
	s.oauthMu.Lock()
	for key, pending := range s.gmailOAuth {
		if now.After(pending.ExpiresAt) {
			delete(s.gmailOAuth, key)
		}
	}
	s.gmailOAuth[state] = gmailOAuthPending{
		ClientID: clientID, ClientSecret: clientSecret, Email: email,
		RedirectURI: redirect, UserID: currentUser(r), ExpiresAt: now.Add(gmailOAuthTTL),
	}
	s.oauthMu.Unlock()

	writeOK(w, map[string]any{
		"auth_url":           gmail.AuthorizationURL(clientID, redirect, state, email),
		"expires_in_seconds": int(gmailOAuthTTL.Seconds()),
	})
}

// GET /api/v1/settings/gmail/oauth/callback
// Google 回调后在服务端换取 Refresh Token，验证 Gmail，再加密入库并热更新。
func (s *Server) handleGmailOAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if oauthErr := strings.TrimSpace(r.URL.Query().Get("error")); oauthErr != "" {
		s.writeGmailOAuthResult(w, false, "Google 授权未完成："+oauthErr, "")
		return
	}
	if state == "" || code == "" {
		s.writeGmailOAuthResult(w, false, "授权回调缺少 state 或 code", "")
		return
	}

	s.oauthMu.Lock()
	pending, ok := s.gmailOAuth[state]
	delete(s.gmailOAuth, state) // authorization code 和 state 都只允许使用一次。
	s.oauthMu.Unlock()
	if !ok || time.Now().After(pending.ExpiresAt) {
		s.writeGmailOAuthResult(w, false, "授权请求已失效，请返回设置页重试", "")
		return
	}
	if pending.UserID == "" || pending.UserID != currentUser(r) {
		s.writeGmailOAuthResult(w, false, "登录会话与授权请求不匹配", "")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	refreshToken, err := gmail.ExchangeCode(ctx, pending.ClientID, pending.ClientSecret, pending.RedirectURI, code)
	if err != nil {
		s.writeGmailOAuthResult(w, false, err.Error(), "")
		return
	}
	client := gmail.NewClient(pending.ClientID, pending.ClientSecret, refreshToken, pending.Email)
	email, err := client.EmailAddress(ctx)
	if err != nil {
		s.writeGmailOAuthResult(w, false, "Gmail 连接测试失败："+err.Error(), "")
		return
	}
	values := map[string]string{
		"client_id": pending.ClientID, "client_secret": pending.ClientSecret,
		"refresh_token": refreshToken, "email": email,
	}
	if err := s.persistIntegrationSettings("gmail", values); err != nil {
		s.writeGmailOAuthResult(w, false, "保存 Gmail 配置失败："+err.Error(), "")
		return
	}
	s.applyIntegrationValues("gmail", values)
	s.writeGmailOAuthResult(w, true, "Gmail 授权成功，配置已保存并立即生效", email)
}

func requestOrigin(baseURL string, r *http.Request) string {
	if configured := strings.TrimRight(strings.TrimSpace(baseURL), "/"); configured != "" {
		return configured
	}
	proto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if proto != "http" && proto != "https" {
		proto = "http"
		if r.TLS != nil {
			proto = "https"
		}
	}
	return proto + "://" + r.Host
}

func (s *Server) writeGmailOAuthResult(w http.ResponseWriter, success bool, message, email string) {
	eventType := "gmail-oauth-error"
	if success {
		eventType = "gmail-oauth-success"
	}
	payload, _ := json.Marshal(map[string]string{"type": eventType, "message": message, "email": email})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><title>Gmail 授权</title><body><p id="message"></p><script>const result=%s;document.getElementById("message").textContent=result.message;if(window.opener){window.opener.postMessage(result,window.location.origin);if(result.type==="gmail-oauth-success")setTimeout(()=>window.close(),500)}</script></body></html>`, payload)
}
