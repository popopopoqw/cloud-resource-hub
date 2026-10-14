// Package r2 实现 Cloudflare R2（S3 兼容）对象上传与读取。
// 使用 AWS Signature V4 + UNSIGNED-PAYLOAD，纯标准库实现，无第三方依赖。
//
// 配置：CF_ACCOUNT_ID / R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY / R2_BUCKET
// 端点：https://{accountID}.r2.cloudflarestorage.com（S3 API）
// 文档：https://developers.cloudflare.com/r2/api/s3/api/
package r2

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	host    = "r2.cloudflarestorage.com"
	region  = "auto"
	service = "s3"
)

// Client R2 客户端。Configured() 为 false 时表示未配置（调用返回错误）。
type Client struct {
	accountID string
	accessKey string
	secretKey string
	bucket    string
	http      *http.Client
}

// New 创建客户端；任一参数为空则处于未配置状态。
func New(accountID, accessKey, secretKey, bucket string) *Client {
	return &Client{
		accountID: accountID,
		accessKey: accessKey,
		secretKey: secretKey,
		bucket:    bucket,
		http:      &http.Client{Timeout: 10 * time.Minute},
	}
}

// Configured 是否已配置完整凭据。
func (c *Client) Configured() bool {
	return c.accountID != "" && c.accessKey != "" && c.secretKey != "" && c.bucket != ""
}

// BucketName 返回桶名（供展示）。
func (c *Client) BucketName() string { return c.bucket }

// Probe 通过列举最多一个对象验证 R2 凭据与目标 Bucket，不写入任何数据。
func (c *Client) Probe(ctx context.Context) error {
	if !c.Configured() {
		return fmt.Errorf("R2 配置不完整")
	}
	endpoint := fmt.Sprintf("https://%s.%s/%s?list-type=2&max-keys=1", c.accountID, host, c.bucket)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	c.sign(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("R2 连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("R2 验证失败 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// PutObject 上传对象。R2 的 S3 端点要求 PUT 携带 Content-Length（拒绝 chunked）：
// size >= 0 时直接设置；未知（-1）时缓冲到内存再上传（电子书场景体积可控）。
// 返回实际上传的对象大小。
func (c *Client) PutObject(key string, r io.Reader, contentType string, size int64) (int64, error) {
	if !c.Configured() {
		return 0, fmt.Errorf("R2 未配置：请设置 CF_ACCOUNT_ID/R2_ACCESS_KEY_ID/R2_SECRET_ACCESS_KEY/R2_BUCKET")
	}
	cr := &countingReader{r: r}
	if size < 0 {
		buf, err := io.ReadAll(io.LimitReader(cr, 512<<20)) // 未知大小：缓冲（上限 512MB）
		if err != nil {
			return cr.n, err
		}
		r = bytes.NewReader(buf)
		cr = &countingReader{r: r}
		size = int64(len(buf))
	}
	endpoint := fmt.Sprintf("https://%s.%s/%s/%s", c.accountID, host, c.bucket, key)
	req, err := http.NewRequest(http.MethodPut, endpoint, cr)
	if err != nil {
		return 0, err
	}
	req.ContentLength = size
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)
	c.sign(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return cr.n, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return cr.n, fmt.Errorf("R2 上传失败 HTTP %d: %s", resp.StatusCode, string(b))
	}
	return cr.n, nil
}

// countingReader 统计实际读取的字节数。
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// GetObject 读取对象（应用内代理下载用）。
func (c *Client) GetObject(key string) (io.ReadCloser, int64, string, error) {
	if !c.Configured() {
		return nil, 0, "", fmt.Errorf("R2 未配置")
	}
	endpoint := fmt.Sprintf("https://%s.%s/%s/%s", c.accountID, host, c.bucket, key)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, "", err
	}
	c.sign(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, "", fmt.Errorf("R2 读取失败 HTTP %d", resp.StatusCode)
	}
	return resp.Body, resp.ContentLength, resp.Header.Get("Content-Type"), nil
}

// ---- AWS SigV4（payload 用 UNSIGNED-PAYLOAD） ----

func (c *Client) sign(req *http.Request) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")

	endpointHost := req.URL.Host
	canonicalURI := req.URL.EscapedPath()
	canonicalQS := req.URL.RawQuery

	// canonical headers（host + x-amz-*，按字典序）
	headers := []string{"host:" + endpointHost, "x-amz-content-sha256:UNSIGNED-PAYLOAD", "x-amz-date:" + amzDate}
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	// method 统一大写（PUT/GET）
	method := strings.ToUpper(req.Method)
	canonicalReq := strings.Join([]string{method, canonicalURI, canonicalQS,
		strings.Join(headers, "\n") + "\n", signedHeaders, "UNSIGNED-PAYLOAD"}, "\n")

	scope := dateStamp + "/" + region + "/" + service + "/aws4_request"
	strToSign := strings.Join([]string{"AWS4-HMAC-SHA256", amzDate, scope, sha256Hex(canonicalReq)}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+c.secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, strToSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.accessKey, scope, signedHeaders, signature))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
