// Gmail OAuth 辅助授权：--gmail-auth 子命令使用。
// 流程（Google 已弃用 OOB，采用 loopback 回调）：
//  1. 起临时 HTTP server 监听 127.0.0.1:{GMAIL_AUTH_PORT:-8484}/callback
//  2. 打印 Google 授权 URL（scope=gmail.readonly, access_type=offline, prompt=consent）
//  3. 用户浏览器完成同意后 Google 302 回 callback?code=...
//  4. 用 code 换 refresh token 并打印，用户填入 GMAIL_REFRESH_TOKEN 环境变量
package gmail

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const oauthScope = "https://www.googleapis.com/auth/gmail.readonly"

// RunAuthCLI 执行交互式授权流程，成功返回 refresh token。
func RunAuthCLI(clientID, clientSecret string) (string, error) {
	port := os.Getenv("GMAIL_AUTH_PORT")
	if port == "" {
		port = "8484"
	}
	// redirect_uri 必须与回调路由一致（/callback），否则 Google 跳回根路径 404
	redirect := "http://127.0.0.1:" + port + "/callback"
	ls, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return "", fmt.Errorf("监听回调端口失败(%s): %w", redirect, err)
	}
	defer ls.Close()

	authorizeURL := AuthorizationURL(clientID, redirect, "", "")

	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "未收到 authorization code，请重试")
			return
		}
		fmt.Fprintln(w, "✅ 授权成功，请回到终端继续。本页面可关闭。")
		select {
		case codeCh <- code:
		default:
		}
	})

	fmt.Println("════════ Gmail OAuth 辅助授权 ════════")
	fmt.Println()
	fmt.Println("步骤 1/2：请在浏览器打开以下链接并登录授权（只读权限）：")
	fmt.Println()
	fmt.Println("  " + authorizeURL)
	fmt.Println()
	fmt.Println("步骤 2/2：授权后浏览器会跳转到本机回调地址，终端将自动继续…")
	fmt.Printf("（回调监听：%s/callback，如端口被占用可用 GMAIL_AUTH_PORT 修改）\n\n", redirect)

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ls) }()

	select {
	case code := <-codeCh:
		_ = srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return ExchangeCode(ctx, clientID, clientSecret, redirect, code)
	case <-time.After(10 * time.Minute):
		_ = srv.Close()
		return "", fmt.Errorf("等待授权超时（10 分钟）")
	}
}

// AuthorizationURL 生成 Google OAuth 授权地址。state 用于防止 CSRF，loginHint 可留空。
func AuthorizationURL(clientID, redirect, state, loginHint string) string {
	values := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirect},
		"response_type": {"code"},
		"scope":         {oauthScope},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}
	if state != "" {
		values.Set("state", state)
	}
	if loginHint != "" {
		values.Set("login_hint", loginHint)
	}
	return authURL + "?" + values.Encode()
}

// ExchangeCode 用 authorization code 换 refresh token。
func ExchangeCode(ctx context.Context, clientID, clientSecret, redirect, code string) (string, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirect},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("换取 token 失败（确认网络可达 googleapis.com，必要时配置 HTTPS_PROXY）: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		RefreshToken     string `json:"refresh_token"`
		AccessToken      string `json:"access_token"`
		ErrorDescription string `json:"error_description"`
	}
	if err := jsonDecode(resp.Body, &out); err != nil {
		return "", fmt.Errorf("响应解析失败: HTTP %d: %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || out.RefreshToken == "" {
		return "", fmt.Errorf("换取 token 失败: HTTP %d %s（提示：authorization code 一次性有效，请重新执行）", resp.StatusCode, out.ErrorDescription)
	}
	return out.RefreshToken, nil
}
