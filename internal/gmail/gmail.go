// Package gmail 实现 Gmail（L1 只读）对接：OAuth2 token 管理、
// 收件箱列表/详情/附件下载，以及 RFC 2822 MIME 解析（纯标准库）。
//
// 文档: https://developers.google.com/gmail/api/reference/rest
// scope: https://www.googleapis.com/auth/gmail.readonly
package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"resource-center/internal/mail"
)

// ---- OAuth2 / REST 客户端 ----

const (
	authURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL = "https://oauth2.googleapis.com/token"
	apiBase  = "https://gmail.googleapis.com/gmail/v1/users/me"
)

// Client Gmail API 客户端（refresh token 换取/缓存 access token）。
type Client struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
	Email        string // 授权账号（展示用）

	http       *http.Client
	mu         sync.Mutex
	accessTok  string
	tokenExpAt time.Time
}

// NewClient 创建客户端。
func NewClient(clientID, clientSecret, refreshToken, email string) *Client {
	return &Client{
		ClientID: clientID, ClientSecret: clientSecret, RefreshToken: refreshToken, Email: email,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// ID 实现 mail.Provider。
func (c *Client) ID() string { return "gmail" }

// Name 实现 mail.Provider。
func (c *Client) Name() string { return "Gmail" }

// Status 实现 mail.Provider。
func (c *Client) Status() map[string]any {
	return map[string]any{"mock": false, "email": c.Email, "scope": "gmail.readonly"}
}

// Probe 用 Gmail profile 轻量接口验证网络与 OAuth 授权。
func (c *Client) Probe(ctx context.Context) error {
	_, err := c.EmailAddress(ctx)
	return err
}

// EmailAddress 返回当前 OAuth 授权对应的 Gmail 地址。
func (c *Client) EmailAddress(ctx context.Context) (string, error) {
	var profile struct {
		EmailAddress string `json:"emailAddress"`
	}
	if err := c.call(ctx, "/profile", &profile); err != nil {
		return "", err
	}
	if profile.EmailAddress == "" {
		return "", fmt.Errorf("Gmail 未返回账号信息")
	}
	return profile.EmailAddress, nil
}

// accessToken 惰性刷新 access token（约 1 小时有效）。
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessTok != "" && time.Now().Before(c.tokenExpAt) {
		return c.accessTok, nil
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.RefreshToken},
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("token 请求失败（检查网络/代理是否可达 googleapis.com）: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error_description"`
	}
	_ = jsonDecode(resp.Body, &out)
	if resp.StatusCode != http.StatusOK || out.AccessToken == "" {
		return "", fmt.Errorf("刷新 token 失败: HTTP %d %s（refresh token 可能已失效，需重新 --gmail-auth）", resp.StatusCode, out.Error)
	}
	c.accessTok = out.AccessToken
	c.tokenExpAt = time.Now().Add(time.Duration(out.ExpiresIn-60) * time.Second)
	return c.accessTok, nil
}

func (c *Client) call(ctx context.Context, path string, out any) error {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("Gmail API HTTP %d: %s", resp.StatusCode, string(b))
	}
	return jsonDecode(resp.Body, out)
}

// ---- Gmail API 原始结构 ----

type rawMessage struct {
	ID           string      `json:"id"`
	ThreadID     string      `json:"threadID"`
	LabelIDs     []string    `json:"labelIds"`
	Snippet      string      `json:"snippet"`
	Payload      *rawPayload `json:"payload"`
	SizeEstimate int64       `json:"sizeEstimate"`
}

type rawPayload struct {
	Headers []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	MimeType string        `json:"mimeType"`
	Body     *rawBody      `json:"body"`
	Parts    []*rawPayload `json:"parts"`
	Filename string        `json:"filename"`
}

type rawBody struct {
	Data         string `json:"data"`
	Size         int64  `json:"size"`
	AttachmentID string `json:"attachmentId"`
}

// List 实现 mail.Provider。
func (c *Client) List(ctx context.Context, q, pageToken string) (*mail.MessageList, error) {
	vs := url.Values{"maxResults": {"25"}}
	if q != "" {
		vs.Set("q", q)
	}
	if pageToken != "" {
		vs.Set("pageToken", pageToken)
	}
	var raw struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		NextPageToken      string `json:"nextPageToken"`
		ResultSizeEstimate int64  `json:"resultSizeEstimate"`
	}
	if err := c.call(ctx, "/messages?"+vs.Encode(), &raw); err != nil {
		return nil, err
	}
	list := &mail.MessageList{NextPageToken: raw.NextPageToken, ResultTotal: raw.ResultSizeEstimate}
	// Gmail 列表只给 ID，逐条拉元数据（format=metadata 太多次调用，
	// 改为每条 format=full；25 条 = 125 quota units，在 250/s 内）
	for _, m := range raw.Messages {
		d, err := c.Get(ctx, m.ID)
		if err != nil {
			continue // 单条失败不阻塞列表
		}
		list.Messages = append(list.Messages, d.MessageSummary)
	}
	if list.Messages == nil {
		list.Messages = []mail.MessageSummary{}
	}
	return list, nil
}

// Get 实现 mail.Provider。
func (c *Client) Get(ctx context.Context, id string) (*mail.MessageDetail, error) {
	var raw rawMessage
	if err := c.call(ctx, "/messages/"+id+"?format=full", &raw); err != nil {
		return nil, err
	}
	d := &mail.MessageDetail{MessageSummary: mail.MessageSummary{
		ID: raw.ID, Snippet: raw.Snippet,
		Unread: hasLabel(raw.LabelIDs, "UNREAD"),
	}}
	for _, h := range raw.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from":
			d.From = decodeMIME(h.Value)
		case "to":
			d.To = decodeMIME(h.Value)
		case "subject":
			d.Subject = decodeMIME(h.Value)
		case "date":
			d.Date = h.Value
		}
	}
	collectParts(raw.Payload, &d.BodyText, &d.BodyHTML, &d.Attachments)
	return d, nil
}

// AttachmentData 实现 mail.Provider。
func (c *Client) AttachmentData(ctx context.Context, msgID, attID string) (io.ReadCloser, string, int64, error) {
	var out struct {
		Data string `json:"data"`
		Size int64  `json:"size"`
	}
	if err := c.call(ctx, fmt.Sprintf("/messages/%s/attachments/%s", msgID, attID), &out); err != nil {
		return nil, "", 0, err
	}
	data, err := base64.URLEncoding.DecodeString(strings.TrimRight(out.Data, "="))
	if err != nil {
		return nil, "", 0, fmt.Errorf("附件解码失败: %w", err)
	}
	return io.NopCloser(strings.NewReader(string(data))), "", out.Size, nil
}

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if l == want {
			return true
		}
	}
	return false
}
