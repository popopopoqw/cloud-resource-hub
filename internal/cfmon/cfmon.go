// Package cfmon 实现 Cloudflare 账户级资源监控：
// 资源清单走 REST API v4（Workers 脚本 / Zones / R2 桶），
// 使用统计走 GraphQL Analytics（Workers 调用、站点访问、R2 存储与请求）。
//
// 认证：API Token（建议只读权限：Account Analytics:Read、Workers Scripts:Read、
// Zone:Read + Zone Analytics:Read、R2:Read）。
// 文档：https://developers.cloudflare.com/api/ / https://developers.cloudflare.com/analytics/
package cfmon

import (
	"context"
	"time"
)

// DayPoint 每日访问点。
type DayPoint struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
}

// WorkerStat 单个 Worker 脚本的调用统计。
type WorkerStat struct {
	Script       string  `json:"script"`
	Requests     int64   `json:"requests"`
	Errors       int64   `json:"errors"`
	CPUTimeP99Ms float64 `json:"cpu_time_p99_ms"`
}

// ZoneStat 单个域名（Zone）的访问统计。
type ZoneStat struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Status         string     `json:"status,omitempty"`
	Requests       int64      `json:"requests"`        // 近 30 天
	Bytes          int64      `json:"bytes"`           // 近 30 天出口带宽
	CachedRequests int64      `json:"cached_requests"` // 缓存命中请求数
	Threats        int64      `json:"threats"`         // 威胁拦截
	PageViews      int64      `json:"page_views"`
	Daily          []DayPoint `json:"daily"` // 近 30 天逐日请求
}

// R2BucketStat 单个 R2 桶用量。
type R2BucketStat struct {
	Name        string `json:"name"`
	ObjectCount int64  `json:"object_count"`
	PayloadSize int64  `json:"payload_size"`
}

// Overview 账户监控总览。
type Overview struct {
	AccountID   string         `json:"account_id"`
	AccountName string         `json:"account_name,omitempty"`
	Workers     []WorkerStat   `json:"workers"`
	Zones       []ZoneStat     `json:"zones"`
	R2Buckets   []R2BucketStat `json:"r2_buckets"`
	Since       string         `json:"since"` // 统计起点（RFC3339）
}

// Monitor 监控数据源（Client 与 Mock 实现）。
type Monitor interface {
	// Status 探针：是否已配置/演示模式。
	Status() map[string]any
	// Probe 验证 Cloudflare API 网络与凭据。
	Probe(ctx context.Context) error
	// Overview 拉取总览（实现方自行缓存）。
	Overview(ctx context.Context) (*Overview, error)
}

// 近 30 天统计窗口。
func sinceDays(days int) string {
	return time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)
}
