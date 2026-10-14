// Package pan 定义网盘 Provider 抽象。每个网盘实现 Parse（解析分享）
// 与 Transfer（转存）两个核心能力；未接入的网盘返回占位实现。
package pan

import (
	"context"
	"errors"

	"resource-center/internal/model"
)

// ErrNotImplemented 功能未实现（占位模块统一返回该错误）。
var ErrNotImplemented = errors.New("该网盘暂未接入，敬请期待")

// Provider 网盘提供方接口。
type Provider interface {
	// ID 网盘标识，对应 model.Provider* 常量。
	ID() string
	// Parse 解析分享链接，返回文件清单。
	Parse(ctx context.Context, shareURL, code string) (*model.ParseResult, error)
	// Transfer 把分享中的文件转存到目标目录，通过 progress 回调推进度（0-100）。
	Transfer(ctx context.Context, req TransferRequest, progress func(pct int)) (*TransferResult, error)
}

// TransferRequest 转存请求。
type TransferRequest struct {
	ShareURL     string
	Code         string
	FileIDs      []string // 为空表示全部转存
	TargetFolder string
}

// TransferResult 转存结果。
type TransferResult struct {
	Files []model.File
	Error string // 部分失败时的提示
}

// Registry Provider 注册表。
type Registry struct {
	providers map[string]Provider
	order     []string
}

// NewRegistry 创建注册表。
func NewRegistry() *Registry {
	return &Registry{providers: map[string]Provider{}}
}

// Register 注册 Provider。
func (r *Registry) Register(p Provider) {
	r.providers[p.ID()] = p
	r.order = append(r.order, p.ID())
}

// Get 按 ID 取 Provider。
func (r *Registry) Get(id string) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

// IDs 返回注册顺序的 ID 列表。
func (r *Registry) IDs() []string { return r.order }
