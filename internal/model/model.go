// Package model 定义全局数据结构。
package model

import "time"

// User 注册用户。
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	PassHash  string    `json:"-"` // pbkdf2 摘要，hex
	Salt      string    `json:"-"` // hex
	CreatedAt time.Time `json:"created_at"`
}

// Session 登录会话（内存存储，重启失效）。
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Provider 网盘供应商标识。
const (
	Provider123Pan = "123pan"
	ProviderQuark  = "quark"  // 占位，后期扩展
	ProviderBaidu  = "baidu"  // 占位，后期扩展
	ProviderAli    = "alipay" // 占位，后期扩展
)

// ProviderMeta 前端展示用的网盘元信息。
type ProviderMeta struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Color   string `json:"color"`
	Icon    string `json:"icon"`
}

// File 分享链接中的文件（目录树用 Children 展开或平铺，这里平铺 + ParentID）。
type File struct {
	ID       string `json:"id"`
	ParentID string `json:"parent_id,omitempty"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	MimeType string `json:"mime_type,omitempty"`
}

// ParseResult 解析分享链接的结果。
type ParseResult struct {
	Provider  string    `json:"provider"`
	ShareURL  string    `json:"share_url"`
	ShareName string    `json:"share_name"`
	Files     []File    `json:"files"`
	TotalSize int64     `json:"total_size"`
	FileCount int       `json:"file_count"`
	ExpiredAt time.Time `json:"expired_at,omitempty"`
}

// 任务状态。
const (
	TaskStatusQueued  = "queued"
	TaskStatusRunning = "running"
	TaskStatusSuccess = "success"
	TaskStatusPartial = "partial"
	TaskStatusFailed  = "failed"
)

// Task 一次转存任务。
type Task struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Provider     string    `json:"provider"`
	ShareURL     string    `json:"share_url"`
	ShareName    string    `json:"share_name"`
	FileIDs      []string  `json:"file_ids"`
	FileCount    int       `json:"file_count"`
	TotalSize    int64     `json:"total_size"`
	TargetFolder string    `json:"target_folder"`
	AccountID    string    `json:"account_id,omitempty"`
	Status       string    `json:"status"`
	Progress     int       `json:"progress"` // 0-100
	Error        string    `json:"error,omitempty"`
	Transferred  []File    `json:"transferred,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
}

// Account 绑定的网盘账号（脱敏存储，只保留令牌摘要）。
type Account struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Provider      string    `json:"provider"`
	Name          string    `json:"name"`
	MaskedSecret  string    `json:"masked_secret"`
	Status        string    `json:"status"` // active / expired / placeholder
	CapacityUsed  int64     `json:"capacity_used"`
	CapacityTotal int64     `json:"capacity_total"`
	CreatedAt     time.Time `json:"created_at"`
}

// Resource 资源广场条目（占位数据，后期对接真实资源源）。
type Resource struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Tag       string    `json:"tag,omitempty"` // 1080P / 4K 等
	Size      int64     `json:"size"`
	ShareURL  string    `json:"share_url"`
	Code      string    `json:"code,omitempty"`
	Provider  string    `json:"provider"`
	Views     int       `json:"views"`
	CreatedAt time.Time `json:"created_at"`
}

// Book 书库搜索结果/详情条目。
type Book struct {
	ID          string   `json:"id"` // Z-Library 内部 bookID
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	Year        string   `json:"year,omitempty"`
	Language    string   `json:"language,omitempty"`
	Publisher   string   `json:"publisher,omitempty"`
	Extension   string   `json:"extension"` // epub / pdf / mobi ...
	SizeText    string   `json:"size_text"` // 站点展示用带单位的体积
	CoverURL    string   `json:"cover,omitempty"`
	Rating      float64  `json:"rating,omitempty"`
	Description string   `json:"description,omitempty"`
	Pages       string   `json:"pages,omitempty"`
}

// BookQuery 书库搜索参数。
type BookQuery struct {
	Keyword string
	Page    int
	Lang    string
	Ext     string
	Sort    string
}

// BookSearchResult 书库搜索结果页。
type BookSearchResult struct {
	Total int    `json:"total"`
	Page  int    `json:"page"`
	Books []Book `json:"books"`
	Quota string `json:"quota,omitempty"` // 站点额度展示，如 "10 / 24h"
}

// 书库转存记录状态。
const (
	BookStatusQueued = "queued"
	BookStatusSaving = "saving"
	BookStatusDone   = "done"
	BookStatusFailed = "failed"
)

// ShelfItem 书架收藏（按用户隔离）。
type ShelfItem struct {
	ID       string    `json:"id"`
	UserID   string    `json:"user_id"`
	BookID   string    `json:"book_id"`
	Title    string    `json:"title"`
	Authors  []string  `json:"authors"`
	Cover    string    `json:"cover,omitempty"`
	Format   string    `json:"format,omitempty"`
	SizeText string    `json:"size_text,omitempty"`
	AddedAt  time.Time `json:"added_at"`
}

// BookRecord 书籍转存记录（下载 → R2 的任务）。
type BookRecord struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	BookID        string    `json:"book_id"`
	Title         string    `json:"title"`
	Authors       []string  `json:"authors"`
	Format        string    `json:"format,omitempty"`
	SizeBytes     int64     `json:"size_bytes"`
	Status        string    `json:"status"`
	R2Key         string    `json:"r2_key,omitempty"`         // R2 对象键，成功后可下载
	StorageKind   string    `json:"storage_kind,omitempty"`   // r2 / local
	StorageBucket string    `json:"storage_bucket,omitempty"` // 书籍上传时选定的 Bucket
	Error         string    `json:"error,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	FinishedAt    time.Time `json:"finished_at,omitempty"`
}

// Stats 首页统计。
type Stats struct {
	TotalTasks   int     `json:"total_tasks"`
	SuccessTasks int     `json:"success_tasks"`
	TotalSize    int64   `json:"total_size"`
	SuccessRate  float64 `json:"success_rate"`
}

// ModuleHealth 概览功能模块的最近一次服务探测结果。
// 结果为全局缓存，不按用户隔离。
type ModuleHealth struct {
	ModuleID  string    `json:"module_id"`
	Available bool      `json:"available"`
	Label     string    `json:"label"`
	Detail    string    `json:"detail,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}
