// Package pan123web 通过 123云盘「安卓客户端/网页」内部 API（www.123pan.com/b/api、
// login.123pan.com）实现列目录/建目录/分享解析/批量转存——不使用官方开放平台。
//
// 端点与签名移植自社区逆向项目（tozy1203/alist-123pan-unlock，AList 生态）：
//   - 登录：POST login.123pan.com/api/user/sign_in（安卓 UA 免验证码，账密换 token）
//   - 鉴权：Authorization: Bearer <token> + 安卓端 headers；401 自动重登
//   - 签名：signPath（CRC32 IEEE + 时间混淆表），GetApi 为 URL 追加签名参数
//   - 分享列表：GET /b/api/share/get?shareKey=&SharePwd=&parentFileId=&Page=N
//   - 转存：POST yun.123pan.com/b/api/restful/goapi/v1/file/copy/save（蓝图端点，待联调）
package pan123web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	bAPI     = "https://yun.123pan.com/b/api" // 联调实测：yun 域（www 域 404）
	loginAPI = "https://login.123pan.com/api/user/sign_in"

	epFileList = bAPI + "/file/list/new"
	epMkdir    = bAPI + "/file/upload_request" // type=1 即创建目录（社区实现）
	epRename   = bAPI + "/file/rename"
	epMove     = bAPI + "/file/mod_pid"
	epTrash    = bAPI + "/file/trash"
	epDownload = bAPI + "/file/download_info"
	epShareGet = bAPI + "/share/get"
	epCopySave = "https://yun.123pan.com/b/api/restful/goapi/v1/file/copy/save"

	androidUA = "123pan/v2.4.0(Android_7.1.2;Xiaomi)"
	authRef   = "https://www.123pan.com/"
	authOrg   = "https://www.123pan.com"
)

// Client 123云盘内部 API 客户端。
// 两种凭据形态（可并存，任一即可用）：
//   - token（PAN123_TOKEN）：注入即用；失效时若有账密则自动续期，否则报错
//   - 账密（PAN123_ACCOUNT/PAN123_PASSWORD）：首次调用自动登录换取 token，
//     此后 401 自动重换——完全自动化，配置一次不再维护
type Client struct {
	token    string
	account  string
	password string
	http     *http.Client
}

// New 创建客户端（token 与可选的续期账密）。
func New(token, account, password string) *Client {
	return &Client{
		token:    strings.TrimSpace(token),
		account:  strings.TrimSpace(account),
		password: strings.TrimSpace(password),
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

// Configured 是否具备任一凭据。
func (c *Client) Configured() bool { return c.token != "" || (c.account != "" && c.password != "") }

// ---- 签名（移植自社区逆向实现，勿改动算法） ----

var signTable = []byte{'a', 'd', 'e', 'f', 'g', 'h', 'l', 'm', 'y', 'i', 'j', 'n', 'o', 'p', 'k', 'q', 'r', 's', 't', 'u', 'b', 'c', 'v', 'w', 's', 'z'}

func signPath(path string) (k string, v string) {
	random := fmt.Sprintf("%.f", math.Round(1e7*rand.Float64()))
	now := time.Now().In(time.FixedZone("CST", 8*3600))
	timestamp := fmt.Sprint(now.Unix())
	nowStr := []byte(now.Format("200601021504"))
	for i := 0; i < len(nowStr); i++ {
		nowStr[i] = signTable[nowStr[i]-48]
	}
	timeSign := fmt.Sprint(crc32.ChecksumIEEE(nowStr))
	data := strings.Join([]string{timestamp, random, path, "web", "3", timeSign}, "|")
	dataSign := fmt.Sprint(crc32.ChecksumIEEE([]byte(data)))
	return timeSign, strings.Join([]string{timestamp, random, dataSign}, "-")
}

// GetApi 为 URL 追加防篡改签名参数。
func GetApi(rawURL string) string {
	u, _ := url.Parse(rawURL)
	q := u.Query()
	k, v := signPath(u.Path)
	q.Add(k, v)
	u.RawQuery = q.Encode()
	return u.String()
}

// ---- 登录（一次性换取 token，供 CLI 使用） ----

// ExchangeToken 用账密一次性换取 token（安卓客户端端点，免验证码）。
func ExchangeToken(account, password string) (string, error) {
	c := &Client{http: &http.Client{Timeout: 60 * time.Second}}
	tok, err := c.exchange(account, password)
	if err != nil {
		return "", err
	}
	return tok, nil
}

func (c *Client) exchange(account, password string) (string, error) {
	var body map[string]any
	if strings.Contains(account, "@") {
		body = map[string]any{"mail": account, "password": password, "type": 2}
	} else {
		body = map[string]any{"passport": account, "password": password, "remember": true}
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, loginAPI, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", androidUA)
	req.Header.Set("Platform", "android")
	req.Header.Set("App-Version", "61")
	req.Header.Set("X-App-Version", "2.4.0")
	req.Header.Set("Origin", authOrg)
	req.Header.Set("Referer", authRef)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &out)
	if out.Code != 200 || out.Data.Token == "" {
		return "", fmt.Errorf("123云盘登录失败: %s", orDefault(out.Message, fmt.Sprintf("code=%d", out.Code)))
	}
	return out.Data.Token, nil
}

// ---- 通用请求（带签名与 401 重登） ----

func (c *Client) do(method, u string, body io.Reader, out any) error {
	if !c.Configured() {
		return fmt.Errorf("未配置 123云盘凭据：.env 填 PAN123_ACCOUNT/PAN123_PASSWORD（全自动续期），或 PAN123_TOKEN（--pan123-token 换取）")
	}
	// 请求可能在 401 后自动续期并重试。先缓存 body，避免第二次请求拿到
	// 已被第一次请求读空的 Reader（目录创建、转存都会受此问题影响）。
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(io.LimitReader(body, 8<<20))
		if err != nil {
			return fmt.Errorf("读取请求体失败: %w", err)
		}
	}
	tok := c.token
	if tok == "" {
		t, err := c.exchange(c.account, c.password) // 账密模式：惰性登录
		if err != nil {
			return err
		}
		c.token, tok = t, t
	}
	for attempt := 0; ; attempt++ {
		var requestBody io.Reader
		if body != nil {
			requestBody = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequest(method, GetApi(u), requestBody)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("User-Agent", androidUA)
		req.Header.Set("Platform", "android")
		req.Header.Set("App-Version", "61")
		req.Header.Set("X-App-Version", "2.4.0")
		req.Header.Set("Origin", authOrg)
		req.Header.Set("Referer", authRef)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json;charset=UTF-8")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("网络请求失败: %w", err)
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode == 401 && attempt == 0 {
			if c.account == "" || c.password == "" {
				return fmt.Errorf("PAN123_TOKEN 已失效且未配置账密：重跑 --pan123-token 换新，或填 PAN123_ACCOUNT/PAN123_PASSWORD 启用自动续期")
			}
			t, err := c.exchange(c.account, c.password) // 全自动续期：401 → 重换 token
			if err != nil {
				return fmt.Errorf("token 自动续期失败: %w", err)
			}
			c.token, tok = t, t
			continue
		}
		if resp.StatusCode == 401 {
			return fmt.Errorf("PAN123_TOKEN 续期后仍 401（账号或密码可能有误）")
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncStr(string(raw), 200))
		}
		if out != nil {
			return json.Unmarshal(raw, out)
		}
		return nil
	}
}

// ---- 文件/目录 ----

// FileEntry 网盘文件/目录（真实响应字段，首字母大写；Type=1 为目录）。
type FileEntry struct {
	FileID   int64  `json:"FileId"`
	Filename string `json:"FileName"`
	Type     int    `json:"Type"`
	Size     int64  `json:"Size"`
	UpdateAt string `json:"UpdateAt"`
}

// IsDir 是否目录。
func (f FileEntry) IsDir() bool { return f.Type == 1 }

// ListDirPage 分页列目录（parentFileId=0 为根）。
// limit 最大为 100，page 从 1 开始。
func (c *Client) ListDirPage(parentFileID int64, limit, page int) ([]FileEntry, int, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}
	// 该内部接口对参数顺序敏感，Page 必须放在末尾，否则会返回空列表。
	u := fmt.Sprintf("%s?driveId=0&limit=%d&next=0&orderBy=file_id&orderDirection=desc&parentFileId=%d&trashed=false&Page=%d",
		epFileList, limit, parentFileID, page)
	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			InfoList []FileEntry `json:"InfoList"`
			Total    int         `json:"Total"`
		} `json:"data"`
	}
	if err := c.do(http.MethodGet, u, nil, &r); err != nil {
		return nil, 0, err
	}
	if r.Code != 0 {
		return nil, 0, fmt.Errorf("123云盘接口错误 code=%d: %s", r.Code, orDefault(r.Message, "请求失败"))
	}
	return r.Data.InfoList, r.Data.Total, nil
}

// ListDir 列目录首批数据，供目录树和路径查找等内部操作使用。
func (c *Client) ListDir(parentFileID int64) ([]FileEntry, error) {
	entries, _, err := c.ListDirPage(parentFileID, 100, 1)
	return entries, err
}

// MkdirP 逐级创建目录路径，返回末级目录 ID。
func (c *Client) MkdirP(path string) (int64, error) {
	parent := int64(0)
	for _, seg := range strings.Split(strings.Trim(path, "/"), "/") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		entries, err := c.ListDir(parent)
		if err != nil {
			return 0, err
		}
		found := int64(-1)
		for _, e := range entries {
			if e.IsDir() && e.Filename == seg {
				found = e.FileID
				break
			}
		}
		if found > 0 {
			parent = found
			continue
		}
		body, _ := json.Marshal(map[string]any{
			"driveId": 0, "etag": "", "fileName": seg,
			"parentFileId": parent, "size": 0, "type": 1,
		})
		var r struct {
			Code int `json:"code"`
			Data struct {
				Info struct {
					FileID int64 `json:"fileId"`
				} `json:"Info"`
			} `json:"data"`
		}
		if err := c.do(http.MethodPost, epMkdir, bytes.NewReader(body), &r); err != nil {
			return 0, err
		}
		if r.Data.Info.FileID == 0 {
			return 0, fmt.Errorf("创建目录「%s」失败 code=%d", seg, r.Code)
		}
		parent = r.Data.Info.FileID
	}
	return parent, nil
}

// ---- 文件管理（Rename/Move/Trash/Download，社区验证端点） ----

// Rename 重命名。
func (c *Client) Rename(fileID int64, name string) error {
	body, _ := json.Marshal(map[string]any{"driveId": 0, "fileId": fileID, "fileName": name})
	var r struct {
		Code int `json:"code"`
	}
	if err := c.do(http.MethodPost, epRename, bytes.NewReader(body), &r); err != nil {
		return err
	}
	return nil
}

// Move 移动到目标目录。
func (c *Client) Move(fileIDs []int64, targetID int64) error {
	list := make([]map[string]any, 0, len(fileIDs))
	for _, id := range fileIDs {
		list = append(list, map[string]any{"FileId": id})
	}
	body, _ := json.Marshal(map[string]any{"fileIdList": list, "parentFileId": targetID})
	var r struct {
		Code int `json:"code"`
	}
	if err := c.do(http.MethodPost, epMove, bytes.NewReader(body), &r); err != nil {
		return err
	}
	return nil
}

// Trash 移入回收站。
func (c *Client) Trash(fileIDs []int64) error {
	list := make([]map[string]any, 0, len(fileIDs))
	for _, id := range fileIDs {
		list = append(list, map[string]any{"FileId": id, "Type": 0})
	}
	body, _ := json.Marshal(map[string]any{"driveId": 0, "operation": true, "fileTrashInfoList": list})
	var r struct {
		Code int `json:"code"`
	}
	if err := c.do(http.MethodPost, epTrash, bytes.NewReader(body), &r); err != nil {
		return err
	}
	return nil
}

// DownloadURL 取文件下载直链（社区实现含 base64 params 解码）。
// TODO(联调)：fileName/size/type 是否必填以真实响应校准（先最小 body）。
func (c *Client) DownloadURL(fileID int64, fileName string, size int64, fileType int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"driveId": 0, "fileId": fileID,
	})
	var r struct {
		Code int `json:"code"`
		Data struct {
			DownloadUrl string `json:"DownloadUrl"`
		} `json:"data"`
	}
	if err := c.do(http.MethodPost, epDownload, bytes.NewReader(body), &r); err != nil {
		return "", err
	}
	u := r.Data.DownloadUrl
	if u == "" {
		return "", fmt.Errorf("未获取到直链")
	}
	// 社区逻辑：params 为 base64 的真实地址时解码
	if pu, _ := url.Parse(u); pu != nil {
		if params := pu.Query().Get("params"); params != "" {
			if dec, err := base64Decode(params); err == nil && strings.HasPrefix(dec, "http") {
				return dec, nil
			}
		}
	}
	return u, nil
}

func base64Decode(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			return "", err
		}
	}
	return string(b), nil
}

// MkdirUnder 在指定父目录下创建子目录，返回新目录 ID。
func (c *Client) MkdirUnder(parentID int64, name string) (int64, error) {
	body, _ := json.Marshal(map[string]any{
		"driveId": 0, "etag": "", "fileName": name,
		"parentFileId": parentID, "size": 0, "type": 1,
	})
	var r struct {
		Code int `json:"code"`
		Data struct {
			Info struct {
				FileID int64 `json:"FileId"`
			} `json:"Info"`
		} `json:"data"`
	}
	if err := c.do(http.MethodPost, epMkdir, bytes.NewReader(body), &r); err != nil {
		return 0, err
	}
	if r.Data.Info.FileID == 0 {
		return 0, fmt.Errorf("创建目录「%s」失败 code=%d", name, r.Code)
	}
	return r.Data.Info.FileID, nil
}

// DirSize 递归统计目录总大小，仅保留作诊断校验。
// 正常列表展示应直接使用 /file/list/new 返回的目录 Size，避免昂贵遍历。
func (c *Client) DirSize(dirID int64) (int64, error) {
	var total int64
	var walk func(id int64, depth int) error
	walk = func(id int64, depth int) error {
		if depth > 6 {
			return nil
		}
		entries, err := c.ListDir(id)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				time.Sleep(120 * time.Millisecond)
				if err := walk(e.FileID, depth+1); err != nil {
					return err
				}
				continue
			}
			total += e.Size
		}
		return nil
	}
	if err := walk(dirID, 0); err != nil {
		return 0, err
	}
	return total, nil
}

// ---- 分享解析与转存 ----

// ShareFile 分享包内文件（响应字段大写；请求体仍用小写，二者独立）。
type ShareFile struct {
	FileID   int64  `json:"FileId"`
	Filename string `json:"FileName"`
	Type     int    `json:"Type"`
	Size     int64  `json:"Size"`
}

// IsDir 是否目录。
func (f ShareFile) IsDir() bool { return f.Type == 1 }

// ShareInfo 解析分享链接（share/get 分页拉全量文件清单）。
func (c *Client) ShareInfo(shareKey, pwd string) ([]ShareFile, error) {
	var all []ShareFile
	for page := 1; ; page++ {
		u := fmt.Sprintf("%s?limit=100&next=0&orderBy=file_id&orderDirection=desc&parentFileId=0&Page=%d&shareKey=%s&SharePwd=%s",
			epShareGet, page, url.QueryEscape(shareKey), url.QueryEscape(pwd))
		var r struct {
			Code int `json:"code"`
			Data struct {
				InfoList []ShareFile `json:"InfoList"`
				Next     string      `json:"Next"`
			} `json:"data"`
		}
		if err := c.do(http.MethodGet, u, nil, &r); err != nil {
			return nil, err
		}
		all = append(all, r.Data.InfoList...)
		if len(r.Data.InfoList) == 0 || r.Data.Next == "-1" {
			break
		}
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("分享包为空或提取码有误")
	}
	return all, nil
}

// CopySave 批量转存（蓝图端点，秒级指针复制）。
// TODO(联调)：请求体字段与该端点的签名要求以真实抓包校准。
func (c *Client) CopySave(shareKey, pwd string, parentFileID int64, files []ShareFile) (int, error) {
	body, count, err := buildCopySaveBody(shareKey, pwd, parentFileID, files)
	if err != nil {
		return 0, err
	}
	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := c.do(http.MethodPost, epCopySave, bytes.NewReader(body), &r); err != nil {
		return 0, err
	}
	if r.Code != 0 {
		return 0, fmt.Errorf("转存失败 code=%d %s", r.Code, r.Message)
	}
	return count, nil
}

func buildCopySaveBody(shareKey, pwd string, parentFileID int64, files []ShareFile) ([]byte, int, error) {
	if strings.TrimSpace(shareKey) == "" {
		return nil, 0, fmt.Errorf("shareKey 不能为空")
	}
	if parentFileID <= 0 {
		return nil, 0, fmt.Errorf("目标目录无效")
	}
	if len(files) == 0 {
		return nil, 0, fmt.Errorf("分享包中没有可转存文件")
	}
	type fl struct {
		FileID       int64 `json:"fileId"`
		ParentFileID int64 `json:"parentFileId"`
	}
	list := make([]fl, 0, len(files))
	for _, f := range files {
		if f.FileID <= 0 {
			continue
		}
		// 123 云盘 copy/save 要求每个源文件同时声明落盘目标目录。
		list = append(list, fl{FileID: f.FileID, ParentFileID: parentFileID})
	}
	if len(list) == 0 {
		return nil, 0, fmt.Errorf("分享包中没有有效文件 ID")
	}
	body, err := json.Marshal(map[string]any{
		"shareKey": shareKey, "sharePwd": pwd,
		"parentFileId": parentFileID, "current_level": 1, "driveId": 0,
		"file_list": list,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("构建转存请求失败: %w", err)
	}
	return body, len(list), nil
}

// ShareKeyOf 从分享 URL 提取 key（…/s/{key}）。
func ShareKeyOf(shareURL string) string {
	u, err := url.Parse(shareURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if (p == "s" || p == "share" || p == "123pan") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// PlanTargetPath 将资源直接归档到既有的“影视合集”分类目录。
// 电视剧按标题里的地区线索分为韩剧、国产剧、美剧；无法识别时进入未分类。
func PlanTargetPath(title, year, mediaType string) string {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch {
	case mediaType == "variety" || mediaType == "综艺" || containsAnyText(title, "综艺", "真人秀", "脱口秀"):
		return "/影视合集/综艺"
	case mediaType == "anime" || mediaType == "动漫":
		return "/影视合集/动漫"
	case mediaType == "documentary" || mediaType == "纪录片":
		return "/影视合集/纪录片"
	case mediaType == "movie" || mediaType == "电影":
		return "/影视合集/电影"
	default:
		return "/影视合集/" + tvTargetCategory(title, mediaType)
	}
}

func tvTargetCategory(title, mediaType string) string {
	if containsAnyText(mediaType, "korea", "korean", "kr", "韩") ||
		containsAnyText(title, "韩剧", "韩国", "韩语", "[韩]", "【韩】") {
		return "韩剧"
	}
	if containsAnyText(mediaType, "american", "usa", "us", "western", "美") ||
		containsAnyText(title, "美剧", "美国", "欧美", "英剧", "英国", "[美]", "【美】", "HBO", "AMC") {
		return "美剧"
	}
	if containsAnyText(mediaType, "china", "chinese", "domestic", "国产") ||
		containsAnyText(title, "国产剧", "国产", "大陆", "内地", "中国", "华语", "港剧", "香港", "台剧", "台湾", "[国]", "【国】") {
		return "国产剧"
	}
	return "未分类"
}

func containsAnyText(s string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(s, value) {
			return true
		}
	}
	return false
}

// ---- 小工具 ----

func truncStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
