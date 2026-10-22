package cfmon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	restBase   = "https://api.cloudflare.com/client/v4"
	graphqlURL = "https://api.cloudflare.com/client/v4/graphql"
)

// Client Cloudflare 监控客户端：REST 清单 + GraphQL 统计，5 分钟内存缓存。
type Client struct {
	apiToken  string
	accountID string
	http      *http.Client
	cacheTTL  time.Duration

	mu        sync.Mutex
	cached    *Overview
	cachedAt  time.Time
	lastError string
}

// NewClient 创建客户端。
func NewClient(apiToken, accountID string) *Client {
	return &Client{
		apiToken:  NormalizeAPIToken(apiToken),
		accountID: accountID,
		http:      &http.Client{Timeout: 30 * time.Second},
		cacheTTL:  5 * time.Minute,
	}
}

// NormalizeAPIToken 接受 Cloudflare 控制台复制出的原始 Token，也兼容用户
// 粘贴完整 Authorization 请求头或 Bearer 前缀。请求客户端会自行添加 Bearer，
// 因此配置中只保留 Token 本体，避免生成 "Bearer Bearer ..." 非法请求头。
func NormalizeAPIToken(value string) string {
	token := strings.TrimSpace(value)
	if len(token) >= len("authorization:") && strings.EqualFold(token[:len("authorization:")], "authorization:") {
		token = strings.TrimSpace(token[len("authorization:"):])
	}
	for {
		fields := strings.Fields(token)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "bearer") {
			break
		}
		token = strings.TrimSpace(strings.Join(fields[1:], " "))
	}
	return token
}

// Status 实现 Monitor。
func (c *Client) Status() map[string]any {
	return map[string]any{"mock": false, "configured": true, "account_id": c.accountID}
}

// Probe 通过账户信息接口轻量验证 Cloudflare 网络与凭据。
func (c *Client) Probe(ctx context.Context) error {
	_, err := c.accountName(ctx)
	return err
}

// Overview 实现 Monitor（带缓存）。
func (c *Client) Overview(ctx context.Context) (*Overview, error) {
	c.mu.Lock()
	if c.cached != nil && time.Since(c.cachedAt) < c.cacheTTL {
		out := c.cached
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()

	o, err := c.fetch(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.cached, c.cachedAt = o, time.Now()
	c.mu.Unlock()
	return o, nil
}

func (c *Client) fetch(ctx context.Context) (*Overview, error) {
	o := &Overview{AccountID: c.accountID, Since: sinceDays(30)}
	var firstErr error
	fail := func(stage string, err error) {
		if firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", stage, err)
		}
	}

	// 1) 账户名
	if name, err := c.accountName(ctx); err == nil {
		o.AccountName = name
	} else {
		fail("账户信息", err)
	}

	// 2) Workers 清单 + 调用统计
	scripts, err := c.workerScripts(ctx)
	if err != nil {
		fail("Workers 清单", err)
	}
	wstats, err := c.workerStats(ctx)
	if err != nil {
		fail("Workers 统计", err)
	}
	statByScript := map[string]WorkerStat{}
	for _, w := range wstats {
		statByScript[w.Script] = w
	}
	o.Workers = []WorkerStat{}
	for _, name := range scripts {
		st := statByScript[name]
		st.Script = name
		o.Workers = append(o.Workers, st)
	}

	// 3) Zones 清单 + 访问统计
	zones, err := c.zoneList(ctx)
	if err != nil {
		fail("域名清单", err)
	}
	o.Zones = []ZoneStat{}
	for _, z := range zones {
		zs, err := c.zoneStats(ctx, z.id, z.name, z.status)
		if err != nil {
			fail("站点统计("+z.name+")", err)
			o.Zones = append(o.Zones, ZoneStat{ID: z.id, Name: z.name, Status: z.status, Daily: []DayPoint{}})
			continue
		}
		o.Zones = append(o.Zones, *zs)
	}

	// 4) R2 桶用量
	if buckets, err := c.r2Stats(ctx); err != nil {
		fail("R2 用量", err)
	} else {
		o.R2Buckets = buckets
	}
	if o.R2Buckets == nil {
		o.R2Buckets = []R2BucketStat{}
	}
	if firstErr != nil && len(o.Workers) == 0 && len(o.Zones) == 0 && len(o.R2Buckets) == 0 {
		return nil, firstErr
	}
	return o, nil
}

// ---- REST v4 ----

type restResp[T any] struct {
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Result T `json:"result"`
}

func (c *Client) restGet(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, restBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("网络不可达（需能访问 api.cloudflare.com）: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) accountName(ctx context.Context) (string, error) {
	var out restResp[struct {
		Name string `json:"name"`
	}]
	if err := c.restGet(ctx, "/accounts/"+c.accountID, &out); err != nil {
		return "", err
	}
	return out.Result.Name, nil
}

func (c *Client) workerScripts(ctx context.Context) ([]string, error) {
	var out restResp[[]struct {
		ID string `json:"id"`
	}]
	if err := c.restGet(ctx, "/accounts/"+c.accountID+"/workers/scripts", &out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Result))
	for _, s := range out.Result {
		names = append(names, s.ID)
	}
	return names, nil
}

func (c *Client) zoneList(ctx context.Context) ([]struct{ id, name, status string }, error) {
	var out restResp[[]struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}]
	// zone 级 token 可能只覆盖部分域名；分页取前 50
	if err := c.restGet(ctx, "/zones?per_page=50", &out); err != nil {
		return nil, err
	}
	zones := make([]struct{ id, name, status string }, 0, len(out.Result))
	for _, z := range out.Result {
		zones = append(zones, struct{ id, name, status string }{z.ID, z.Name, z.Status})
	}
	return zones, nil
}

// ---- GraphQL Analytics ----

// gqlResp GraphQL 响应（多查询合并时的通用解析）。
type gqlResp struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) graphql(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("网络不可达（需能访问 api.cloudflare.com）: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var gr gqlResp
	if err := json.Unmarshal(b, &gr); err != nil {
		return fmt.Errorf("响应解析失败: %v", string(b[:min(len(b), 300)]))
	}
	if len(gr.Errors) > 0 {
		return fmt.Errorf("GraphQL: %s", gr.Errors[0].Message)
	}
	if len(gr.Data) == 0 {
		return fmt.Errorf("GraphQL 空响应")
	}
	return json.Unmarshal(gr.Data, out)
}

const qWorkers = `query($account: String!, $since: Time!) {
  viewer { accounts(filter: {accountTag: $account}) {
    workersInvocationsAdaptiveGroups(limit: 100, filter: {datetime_geq: $since}, orderBy: [sum_requests_DESC]) {
      dimensions { scriptName }
      sum { requests errors }
      quantiles { cpuTimeP99 }
    }
  } } }`

func (c *Client) workerStats(ctx context.Context) ([]WorkerStat, error) {
	var out struct {
		Viewer struct {
			Accounts []struct {
				Groups []struct {
					Dimensions struct {
						ScriptName string `json:"scriptName"`
					} `json:"dimensions"`
					Sum struct {
						Requests int64 `json:"requests"`
						Errors   int64 `json:"errors"`
					} `json:"sum"`
					Quantiles struct {
						CPUTimeP99 float64 `json:"cpuTimeP99"`
					} `json:"quantiles"`
				} `json:"workersInvocationsAdaptiveGroups"`
			} `json:"accounts"`
		} `json:"viewer"`
	}
	if err := c.graphql(ctx, qWorkers, map[string]any{"account": c.accountID, "since": sinceDays(30)}, &out); err != nil {
		return nil, err
	}
	var stats []WorkerStat
	if len(out.Viewer.Accounts) > 0 {
		for _, g := range out.Viewer.Accounts[0].Groups {
			stats = append(stats, WorkerStat{
				Script: g.Dimensions.ScriptName, Requests: g.Sum.Requests,
				Errors: g.Sum.Errors, CPUTimeP99Ms: g.Quantiles.CPUTimeP99,
			})
		}
	}
	return stats, nil
}

const qZone = `query($zone: String!, $since: Date!) {
  viewer { zones(filter: {zoneTag: $zone}) {
    httpRequests1dGroups(limit: 30, filter: {date_geq: $since}, orderBy: [date_ASC]) {
      dimensions { date }
      sum { requests bytes cachedRequests threats pageViews }
    }
  } } }`

func (c *Client) zoneStats(ctx context.Context, zoneID, name, status string) (*ZoneStat, error) {
	since := time.Now().AddDate(0, 0, -30).UTC().Format("2006-01-02")
	var out struct {
		Viewer struct {
			Zones []struct {
				Groups []struct {
					Dimensions struct {
						Date string `json:"date"`
					} `json:"dimensions"`
					Sum struct {
						Requests       int64 `json:"requests"`
						Bytes          int64 `json:"bytes"`
						CachedRequests int64 `json:"cachedRequests"`
						Threats        int64 `json:"threats"`
						PageViews      int64 `json:"pageViews"`
					} `json:"sum"`
				} `json:"httpRequests1dGroups"`
			} `json:"zones"`
		} `json:"viewer"`
	}
	if err := c.graphql(ctx, qZone, map[string]any{"zone": zoneID, "since": since}, &out); err != nil {
		return nil, err
	}
	zs := &ZoneStat{ID: zoneID, Name: name, Status: status, Daily: []DayPoint{}}
	if len(out.Viewer.Zones) > 0 {
		for _, g := range out.Viewer.Zones[0].Groups {
			zs.Requests += g.Sum.Requests
			zs.Bytes += g.Sum.Bytes
			zs.CachedRequests += g.Sum.CachedRequests
			zs.Threats += g.Sum.Threats
			zs.PageViews += g.Sum.PageViews
			zs.Daily = append(zs.Daily, DayPoint{Date: g.Dimensions.Date, Requests: g.Sum.Requests})
		}
	}
	return zs, nil
}

const qR2 = `query($account: String!, $since: Time!) {
  viewer { accounts(filter: {accountTag: $account}) {
    r2StorageAdaptiveGroups(limit: 50, filter: {datetime_geq: $since}) {
      dimensions { bucketName }
      max { objectCount payloadSize }
    }
  } } }`

func (c *Client) r2Stats(ctx context.Context) ([]R2BucketStat, error) {
	var out struct {
		Viewer struct {
			Accounts []struct {
				Groups []struct {
					Dimensions struct {
						BucketName string `json:"bucketName"`
					} `json:"dimensions"`
					Max struct {
						ObjectCount int64 `json:"objectCount"`
						PayloadSize int64 `json:"payloadSize"`
					} `json:"max"`
				} `json:"r2StorageAdaptiveGroups"`
			} `json:"accounts"`
		} `json:"viewer"`
	}
	if err := c.graphql(ctx, qR2, map[string]any{"account": c.accountID, "since": sinceDays(30)}, &out); err != nil {
		return nil, err
	}
	// 同桶多窗口取最大值合并
	byBucket := map[string]R2BucketStat{}
	if len(out.Viewer.Accounts) > 0 {
		for _, g := range out.Viewer.Accounts[0].Groups {
			cur := byBucket[g.Dimensions.BucketName]
			if g.Max.ObjectCount > cur.ObjectCount {
				cur.ObjectCount = g.Max.ObjectCount
			}
			if g.Max.PayloadSize > cur.PayloadSize {
				cur.PayloadSize = g.Max.PayloadSize
			}
			cur.Name = g.Dimensions.BucketName
			byBucket[g.Dimensions.BucketName] = cur
		}
	}
	buckets := make([]R2BucketStat, 0, len(byBucket))
	for _, b := range byBucket {
		buckets = append(buckets, b)
	}
	return buckets, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
