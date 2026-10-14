package server

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"resource-center/internal/model"
	"resource-center/internal/pan"
	"resource-center/internal/store"
)

// io_LimitReader 限制请求体大小。
func io_LimitReader(r *http.Request) io.Reader { return io.LimitReader(r.Body, 1<<20) }

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeOK(w, map[string]any{
		"status":         "up",
		"time":           time.Now().Format(time.RFC3339),
		"started_at":     s.startedAt.Format(time.RFC3339),
		"uptime_seconds": int(time.Since(s.startedAt).Seconds()),
	})
}

// handleConfig 返回前端渲染所需的站点配置（网盘列表、功能开关）。
func (s *Server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	metas := []model.ProviderMeta{
		{ID: model.Provider123Pan, Name: "123云盘", Enabled: true, Color: "#2b5aec", Icon: "123"},
		{ID: model.ProviderQuark, Name: "夸克网盘", Enabled: false, Color: "#5c4dff", Icon: "QK"},
		{ID: model.ProviderBaidu, Name: "百度网盘", Enabled: false, Color: "#2b6de8", Icon: "BD"},
		{ID: model.ProviderAli, Name: "阿里云盘", Enabled: false, Color: "#ff6a00", Icon: "AL"},
	}
	// 网盘列表按注册表如实反映；未配置任何 Provider 时各网盘均为不可用
	for i := range metas {
		if _, ok := s.reg.Get(metas[i].ID); ok {
			metas[i].Enabled = true
		}
	}
	writeOK(w, map[string]any{
		"providers": metas,
		"base_url":  s.cfg.BaseURL,
		"features": map[string]bool{
			"parse": true, "transfer": true, "share": false, "search": false, "oauth_login": false,
		},
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.store.Stats(currentUser(r)))
}

// parseReq 解析请求。
type parseReq struct {
	Provider string `json:"provider"`
	ShareURL string `json:"share_url"`
	Code     string `json:"code"`
}

func (s *Server) handleParse(w http.ResponseWriter, r *http.Request) {
	var req parseReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ShareURL) == "" {
		writeErr(w, http.StatusBadRequest, "分享链接不能为空")
		return
	}
	if req.Provider == "" {
		req.Provider = model.Provider123Pan
	}
	p, ok := s.reg.Get(req.Provider)
	if !ok {
		writeErr(w, http.StatusBadRequest, "网盘 "+req.Provider+" 未接入：请到设置中心配置网盘账号，或使用资源广场的转存通道")
		return
	}
	result, err := p.Parse(r.Context(), req.ShareURL, req.Code)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, result)
}

// transferReq 转存请求。
type transferReq struct {
	Provider     string   `json:"provider"`
	ShareURL     string   `json:"share_url"`
	Code         string   `json:"code"`
	ShareName    string   `json:"share_name"`
	FileIDs      []string `json:"file_ids"`
	FileCount    int      `json:"file_count"`
	TotalSize    int64    `json:"total_size"`
	TargetFolder string   `json:"target_folder"`
	AccountID    string   `json:"account_id"`
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req transferReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.ShareURL) == "" {
		writeErr(w, http.StatusBadRequest, "分享链接不能为空")
		return
	}
	if req.Provider == "" {
		req.Provider = model.Provider123Pan
	}
	if req.TargetFolder == "" {
		req.TargetFolder = "/来自：分享转存"
	}
	if _, ok := s.reg.Get(req.Provider); !ok {
		writeErr(w, http.StatusBadRequest, "网盘 "+req.Provider+" 未接入：请到设置中心配置网盘账号，或使用资源广场的转存通道")
		return
	}

	task := &model.Task{
		ID:           newID("task"),
		UserID:       currentUser(r),
		Provider:     req.Provider,
		ShareURL:     req.ShareURL,
		ShareName:    req.ShareName,
		FileIDs:      req.FileIDs,
		FileCount:    req.FileCount,
		TotalSize:    req.TotalSize,
		TargetFolder: req.TargetFolder,
		AccountID:    req.AccountID,
		Status:       model.TaskStatusQueued,
		Progress:     0,
		CreatedAt:    time.Now(),
	}
	if len(task.FileIDs) > 0 && task.FileCount == 0 {
		task.FileCount = len(task.FileIDs)
	}
	if err := s.store.CreateTask(task); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.startTransferTask(task, req)
	writeOK(w, task)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	status := r.URL.Query().Get("status")
	tasks, total, err := s.store.ListTasks(currentUser(r), status, page, pageSize)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]any{"list": tasks, "total": total, "page": page, "page_size": pageSize})
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.store.GetTask(currentUser(r), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "任务不存在")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, t)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTask(currentUser(r), r.PathValue("id")); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "任务不存在")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, nil)
}

// handleCreateShare 占位：转存成功后把文件再生成分享链接。
// TODO: 对接 123pan 开放平台分享接口后实现。
func (s *Server) handleCreateShare(w http.ResponseWriter, _ *http.Request) {
	_ = pan.ErrNotImplemented
	writeErr(w, http.StatusNotImplemented, "生成分享链接功能规划中，敬请期待")
}
