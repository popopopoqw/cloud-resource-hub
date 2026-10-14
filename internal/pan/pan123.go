package pan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"resource-center/internal/model"
)

const pan123PlatformHeader = "open_platform"

// Pan123 123云盘开放平台客户端。
// 文档: https://123pan.com/a/dokuforopen (开放平台 API 参考)
//
// 当前实现了获取 access_token 与列目录骨架；分享解析/转存接口
// 开放平台暂未完整开放，标记为 TODO，返回 ErrNotImplemented。
type Pan123 struct {
	BaseURL string
	Client  *http.Client

	clientID     string
	clientSecret string
	accessToken  string // 可通过 PAN123_ACCESS_TOKEN 直接注入，跳过签名获取
	tokenExpAt   time.Time
}

// NewPan123 从配置创建客户端。
func NewPan123(baseURL, clientID, clientSecret string) *Pan123 {
	return &Pan123{
		BaseURL:      baseURL,
		Client:       &http.Client{Timeout: 30 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
		accessToken:  os.Getenv("PAN123_ACCESS_TOKEN"),
	}
}

// ID 返回网盘标识。
func (p *Pan123) ID() string { return model.Provider123Pan }

// apiResp 开放平台统一响应结构。
type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (p *Pan123) do(ctx context.Context, method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.BaseURL+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Platform", pan123PlatformHeader)
	req.Header.Set("Content-Type", "application/json")
	if p.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+p.accessToken)
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var r apiResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("响应解析失败(HTTP %d): %s", resp.StatusCode, trunc(string(raw), 200))
	}
	if r.Code != 0 {
		return fmt.Errorf("网盘接口错误 code=%d: %s", r.Code, r.Message)
	}
	if out != nil && len(r.Data) > 0 {
		return json.Unmarshal(r.Data, out)
	}
	return nil
}

// RefreshToken 获取 access_token。
// TODO: 开放平台要求对 clientID/clientSecret 做签名后调用
// POST /api/v1/access_token (grantType=client_credentials)。签名算法
// 以官方文档为准，当前版本请通过环境变量 PAN123_ACCESS_TOKEN 注入令牌。
func (p *Pan123) RefreshToken(ctx context.Context) error {
	if p.accessToken != "" && time.Now().Before(p.tokenExpAt) {
		return nil
	}
	return fmt.Errorf("access_token 获取尚未实现：请设置 PAN123_ACCESS_TOKEN 或补充签名逻辑")
}

// ListFiles 列出网盘目录（分页）。
func (p *Pan123) ListFiles(ctx context.Context, parentID int64, limit int) ([]Pan123File, error) {
	if err := p.RefreshToken(ctx); err != nil {
		return nil, err
	}
	var data struct {
		FileList []Pan123File `json:"InfoList"`
	}
	path := fmt.Sprintf("/api/v1/file/list?parentFileId=%d&limit=%d", parentID, limit)
	if err := p.do(ctx, http.MethodGet, path, nil, &data); err != nil {
		return nil, err
	}
	return data.FileList, nil
}

// Parse 解析分享链接。
// TODO: 等开放平台开放分享信息接口后补全（当前返回占位错误）。
func (p *Pan123) Parse(_ context.Context, _, _ string) (*model.ParseResult, error) {
	return nil, ErrNotImplemented
}

// Transfer 转存分享文件到自己网盘。
// TODO: 等开放平台开放分享转存接口后补全（当前返回占位错误）。
func (p *Pan123) Transfer(_ context.Context, _ TransferRequest, _ func(int)) (*TransferResult, error) {
	return nil, ErrNotImplemented
}

// ---- FS 接口实现（我的网盘） ----
// ⚠️ 端点与字段名按开放平台文档预置，凭据联调时校准（各方法 TODO 标注）。

// ListDir 列出目录内容（分页）。
// TODO(联调)：分页参数（lastFileId 或 page）与响应字段名以实际返回为准。
func (p *Pan123) ListDir(ctx context.Context, dirID int64, page int) (*DirListing, error) {
	if err := p.RefreshToken(ctx); err != nil {
		return nil, err
	}
	const limit = 50
	var data struct {
		FileList []Pan123File `json:"InfoList"`
		LastFile int64        `json:"LastFileId"`
	}
	path := fmt.Sprintf("/api/v1/file/list?parentFileId=%d&limit=%d&page=%d", dirID, limit, page)
	if err := p.do(ctx, http.MethodGet, path, nil, &data); err != nil {
		return nil, err
	}
	list := &DirListing{DirID: dirID, Page: page, PageSize: limit, HasMore: len(data.FileList) >= limit}
	for _, f := range data.FileList {
		list.Entries = append(list.Entries, DirEntry{
			FileID: f.FileID, Name: f.Filename, IsDir: f.Type == 1,
			Size: f.Size, UpdateAt: f.UpdateAt,
		})
	}
	if list.Entries == nil {
		list.Entries = []DirEntry{}
	}
	return list, nil
}

// DirTree 目录树（逐层拉取文件夹）。
// TODO(联调)：开放平台无整树接口，逐层调用；目录层级深时较慢，可加缓存。
func (p *Pan123) DirTree(ctx context.Context, root int64) (*DirNode, error) {
	if err := p.RefreshToken(ctx); err != nil {
		return nil, err
	}
	rootName := "我的网盘"
	if root != 0 {
		var data struct {
			Name string `json:"Name"`
		}
		if err := p.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/file/info?FileId=%d", root), nil, &data); err != nil {
			return nil, err
		}
		rootName = data.Name
	}
	var build func(parent int64) ([]DirNode, error)
	build = func(parent int64) ([]DirNode, error) {
		listing, err := p.ListDir(ctx, parent, 1)
		if err != nil {
			return nil, err
		}
		var nodes []DirNode
		for _, e := range listing.Entries {
			if !e.IsDir {
				continue
			}
			children, err := build(e.FileID)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, DirNode{FileID: e.FileID, Name: e.Name, Dirs: children})
		}
		return nodes, nil
	}
	dirs, err := build(root)
	if err != nil {
		return nil, err
	}
	return &DirNode{FileID: root, Name: rootName, Dirs: dirs}, nil
}

// Mkdir 新建文件夹。
// TODO(联调)：创建目录端点为 POST /upload/v1/file（parentID+名称），
// 响应中的新目录 ID 字段名以实际返回为准。
func (p *Pan123) Mkdir(ctx context.Context, parentID int64, name string) (int64, error) {
	if err := p.RefreshToken(ctx); err != nil {
		return 0, err
	}
	var data struct {
		FileID int64 `json:"FileId"`
		Info   struct {
			FileID int64 `json:"FileId"`
		} `json:"Info"`
	}
	body := map[string]any{"parentID": parentID, "name": name}
	if err := p.do(ctx, http.MethodPost, "/upload/v1/file", body, &data); err != nil {
		return 0, err
	}
	if data.FileID != 0 {
		return data.FileID, nil
	}
	return data.Info.FileID, nil
}

// Rename 重命名。
// TODO(联调)：端点 PUT /api/v1/file/rename，请求体字段（FileId/FileName）联调校准。
func (p *Pan123) Rename(ctx context.Context, fileID int64, name string) error {
	if err := p.RefreshToken(ctx); err != nil {
		return err
	}
	return p.do(ctx, http.MethodPut, "/api/v1/file/rename",
		map[string]any{"FileId": fileID, "FileName": name}, nil)
}

// Move 移动。
// TODO(联调)：端点 PUT /api/v1/file/move，请求体字段（FileIds/ToFileId）联调校准。
func (p *Pan123) Move(ctx context.Context, fileIDs []int64, targetID int64) error {
	if err := p.RefreshToken(ctx); err != nil {
		return err
	}
	return p.do(ctx, http.MethodPut, "/api/v1/file/move",
		map[string]any{"FileIds": fileIDs, "ToFileId": targetID}, nil)
}

// Trash 移入回收站。
// TODO(联调)：端点 POST /api/v1/file/trash，请求体字段联调校准。
func (p *Pan123) Trash(ctx context.Context, fileIDs []int64) error {
	if err := p.RefreshToken(ctx); err != nil {
		return err
	}
	return p.do(ctx, http.MethodPost, "/api/v1/file/trash",
		map[string]any{"FileIds": fileIDs}, nil)
}

// DownloadURL 文件直链。
// TODO(联调)：端点 GET /api/v1/file/download_info?FileId=，直链字段联调校准。
func (p *Pan123) DownloadURL(ctx context.Context, fileID int64) (string, error) {
	if err := p.RefreshToken(ctx); err != nil {
		return "", err
	}
	var data struct {
		URL string `json:"DownloadUrl"`
	}
	if err := p.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/file/download_info?FileId=%d", fileID), nil, &data); err != nil {
		return "", err
	}
	if data.URL == "" {
		return "", fmt.Errorf("未获取到直链")
	}
	return data.URL, nil
}

// Capacity 容量。
// TODO(联调)：端点 GET /api/v1/user/capacity（或 user/info），字段联调校准。
func (p *Pan123) Capacity(ctx context.Context) (int64, int64, error) {
	if err := p.RefreshToken(ctx); err != nil {
		return 0, 0, err
	}
	var data struct {
		Used  int64 `json:"UseCapacity"`
		Total int64 `json:"TotalCapacity"`
	}
	if err := p.do(ctx, http.MethodGet, "/api/v1/user/capacity", nil, &data); err != nil {
		return 0, 0, err
	}
	return data.Used, data.Total, nil
}

// Pan123File 开放平台文件对象（部分字段）。
type Pan123File struct {
	FileID   int64  `json:"FileId"`
	Filename string `json:"Filename"`
	Type     int    `json:"Type"` // 0=文件 1=目录
	Size     int64  `json:"Size"`
	UpdateAt string `json:"UpdateAt"`
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
