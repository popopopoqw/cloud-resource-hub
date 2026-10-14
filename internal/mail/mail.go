// Package mail 定义邮箱供应商抽象（多供应商注册表）。
// Gmail 是第一个实现；Outlook（Microsoft Graph）后期按同一接口接入，
// 前端/API 通过 provider 维度自动获得新邮箱。
package mail

import (
	"context"
	"io"
)

// 消息/附件模型与具体供应商无关。

// MessageSummary 列表条目。
type MessageSummary struct {
	ID            string `json:"id"`
	From          string `json:"from"`
	Subject       string `json:"subject"`
	Snippet       string `json:"snippet"`
	Date          string `json:"date"`
	Unread        bool   `json:"unread"`
	HasAttachment bool   `json:"has_attachment"`
}

// MessageList 列表结果。
type MessageList struct {
	Messages      []MessageSummary `json:"messages"`
	NextPageToken string           `json:"next_page_token"`
	ResultTotal   int64            `json:"result_total"`
}

// Attachment 附件元数据。
type Attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

// MessageDetail 详情。
type MessageDetail struct {
	MessageSummary
	To          string       `json:"to"`
	BodyText    string       `json:"body_text"`
	BodyHTML    string       `json:"body_html,omitempty"`
	Attachments []Attachment `json:"attachments"`
}

// Provider 邮箱供应商接口（L1 只读）。
type Provider interface {
	// ID 供应商标识（URL 与前端切换用），如 "gmail" / "outlook"。
	ID() string
	// Name 展示名，如 "Gmail" / "Outlook"。
	Name() string
	// Status 授权/模式状态。
	Status() map[string]any
	// Probe 验证已配置邮箱服务的网络与授权是否可用。
	Probe(ctx context.Context) error
	// List 列表。q 支持供应商原生搜索语法。
	List(ctx context.Context, q, pageToken string) (*MessageList, error)
	// Get 详情。
	Get(ctx context.Context, id string) (*MessageDetail, error)
	// AttachmentData 附件内容。
	AttachmentData(ctx context.Context, msgID, attID string) (io.ReadCloser, string, int64, error)
}

// Registry 供应商注册表。
type Registry struct {
	providers map[string]Provider
	order     []string
}

// NewRegistry 创建注册表。
func NewRegistry() *Registry { return &Registry{providers: map[string]Provider{}} }

// Register 注册供应商。
func (r *Registry) Register(p Provider) {
	if _, ok := r.providers[p.ID()]; ok {
		return
	}
	r.providers[p.ID()] = p
	r.order = append(r.order, p.ID())
}

// Get 按 ID 取供应商。
func (r *Registry) Get(id string) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

// Default 默认供应商（第一个注册的）。
func (r *Registry) Default() Provider {
	if len(r.order) == 0 {
		return nil
	}
	return r.providers[r.order[0]]
}

// IDs 注册顺序的 ID 列表。
func (r *Registry) IDs() []string { return r.order }
