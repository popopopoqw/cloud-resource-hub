package r2

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
)

// ListBuckets 使用 R2 S3 凭据读取当前账户默认辖区的 Bucket 列表。
// 与对象读写不同，列举 Bucket 需要账户级权限。
func (c *Client) ListBuckets(ctx context.Context) ([]string, error) {
	if c.accountID == "" || c.accessKey == "" || c.secretKey == "" {
		return nil, fmt.Errorf("R2 Account ID、Access Key ID 和 Secret Access Key 必须完整")
	}
	names := map[string]struct{}{}
	continuation := ""
	for page := 0; page < 20; page++ {
		query := url.Values{"max-keys": {"1000"}}
		if continuation != "" {
			query.Set("continuation-token", continuation)
		}
		endpoint := fmt.Sprintf("https://%s.%s/?%s", c.accountID, host, query.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		c.sign(req)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("读取 R2 Bucket 列表失败: %w", err)
		}
		if resp.StatusCode/100 != 2 {
			resp.Body.Close()
			return nil, fmt.Errorf("R2 列举 Bucket 失败 HTTP %d", resp.StatusCode)
		}
		var result struct {
			Buckets struct {
				Items []struct {
					Name string `xml:"Name"`
				} `xml:"Bucket"`
			} `xml:"Buckets"`
			IsTruncated           bool   `xml:"IsTruncated"`
			NextContinuationToken string `xml:"NextContinuationToken"`
		}
		err = xml.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("解析 R2 Bucket 列表失败: %w", err)
		}
		for _, bucket := range result.Buckets.Items {
			if bucket.Name != "" {
				names[bucket.Name] = struct{}{}
			}
		}
		if !result.IsTruncated {
			return sortedBucketNames(names), nil
		}
		if result.NextContinuationToken == "" || result.NextContinuationToken == continuation {
			return nil, fmt.Errorf("R2 Bucket 列表分页信息缺失")
		}
		continuation = result.NextContinuationToken
	}
	return nil, fmt.Errorf("R2 Bucket 数量超过可读取上限")
}

// ListBucketsViaAPI 使用 Cloudflare API Token 列举 Bucket，供 S3 凭据只有桶级权限时回退。
func (c *Client) ListBucketsViaAPI(ctx context.Context, apiToken string) ([]string, error) {
	if c.accountID == "" || apiToken == "" {
		return nil, fmt.Errorf("Cloudflare Account ID 和 API Token 必须完整")
	}
	names := map[string]struct{}{}
	cursor := ""
	for page := 0; page < 20; page++ {
		query := url.Values{"per_page": {"1000"}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/r2/buckets?%s", url.PathEscape(c.accountID), query.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiToken)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("读取 Cloudflare Bucket 列表失败: %w", err)
		}
		var result struct {
			Success bool `json:"success"`
			Errors  []struct {
				Message string `json:"message"`
			} `json:"errors"`
			Result struct {
				Buckets []struct {
					Name         string `json:"name"`
					Jurisdiction string `json:"jurisdiction"`
				} `json:"buckets"`
			} `json:"result"`
			ResultInfo struct {
				Cursor string `json:"cursor"`
			} `json:"result_info"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("解析 Cloudflare Bucket 列表失败: %w", err)
		}
		if resp.StatusCode/100 != 2 || !result.Success {
			detail := ""
			if len(result.Errors) > 0 {
				detail = result.Errors[0].Message
			}
			return nil, fmt.Errorf("Cloudflare 列举 Bucket 失败 HTTP %d: %s", resp.StatusCode, detail)
		}
		for _, bucket := range result.Result.Buckets {
			// 当前对象读写客户端使用默认辖区端点，仅展示能够作为目标的 Bucket。
			if bucket.Name != "" && (bucket.Jurisdiction == "" || bucket.Jurisdiction == "default") {
				names[bucket.Name] = struct{}{}
			}
		}
		if result.ResultInfo.Cursor == "" {
			return sortedBucketNames(names), nil
		}
		if result.ResultInfo.Cursor == cursor {
			return nil, fmt.Errorf("Cloudflare Bucket 列表分页信息重复")
		}
		cursor = result.ResultInfo.Cursor
	}
	return nil, fmt.Errorf("Cloudflare Bucket 数量超过可读取上限")
}

func sortedBucketNames(names map[string]struct{}) []string {
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
