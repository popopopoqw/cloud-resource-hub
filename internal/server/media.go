package server

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"resource-center/internal/model"
	"resource-center/internal/pan123web"
	"resource-center/internal/respool"
)

// ---- 影视全链路（蓝图模块 1-3）：论坛搜索 → 解锁提取 → 网盘落盘 ----

// GET /api/v1/media/search?q=片名&source=pan123fx
// 论坛站内搜索（需登录态），返回匹配的资源帖。
func (s *Server) handleMediaSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeErr(w, http.StatusBadRequest, "请输入片名关键词")
		return
	}
	srcID := r.URL.Query().Get("source")
	if srcID == "" {
		srcID = "pan123fx"
	}
	threads, err := s.resourcePool().Search(srcID, q)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if cat := r.URL.Query().Get("category"); cat != "" && cat != "全部" {
		filtered := make([]respool.Thread, 0, len(threads))
		for _, t := range threads {
			if t.Category == cat {
				filtered = append(filtered, t)
			}
		}
		threads = filtered
	}
	if len(threads) > resourceMaxItems {
		threads = threads[:resourceMaxItems]
	}
	writeOK(w, map[string]any{"list": threads, "source": srcID, "query": q, "limit": resourceMaxItems})
}

var reYear = regexp.MustCompile(`\((19|20)\d{2}\)`)

// POST /api/v1/media/transfer
// {"tid": "12345", "source": "pan123fx", "media_type": "tv", "title": "繁花", "year": "2023"}
// 状态机：探测原帖 → (未解锁)冷却+自动回帖 → 提取分享链接 →
// 规划分类目录 → copy/save 批量转存（网盘端秒级指针复制）。
func (s *Server) handleMediaTransfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TID       string `json:"tid"`
		Source    string `json:"source"`
		Title     string `json:"title"`
		Year      string `json:"year"`
		MediaType string `json:"media_type"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.TID == "" {
		writeErr(w, http.StatusBadRequest, "tid 不能为空")
		return
	}
	if req.Source == "" {
		req.Source = "pan123fx"
	}
	if req.Title == "" {
		req.Title = "未命名资源"
	}
	targetPath := PlanTargetPath(req.Title, req.Year, req.MediaType)
	task := &model.Task{
		ID: newID("task"), UserID: currentUser(r), Provider: model.Provider123Pan,
		ShareName: req.Title, TargetFolder: targetPath,
		Status: model.TaskStatusQueued, CreatedAt: time.Now(),
	}
	if err := s.store.CreateTask(task); err != nil {
		writeErr(w, http.StatusInternalServerError, "创建转存任务失败: "+err.Error())
		return
	}
	fail := func(status int, err error) {
		task.Status = model.TaskStatusFailed
		task.Error = err.Error()
		task.Progress = 100
		task.FinishedAt = time.Now()
		_ = s.store.UpdateTask(task)
		writeErr(w, status, err.Error())
	}
	task.Status = model.TaskStatusRunning
	task.Progress = 10
	_ = s.store.UpdateTask(task)

	// 1) 探测 + 解锁（含防封冷却语义：冷却中返回 429 文案错误）
	thread, err := s.resourcePool().UnlockAndFetch(req.Source, req.TID)
	if err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "冷却") {
			status = http.StatusTooManyRequests
		}
		fail(status, err)
		return
	}
	if len(thread.Shares) == 0 {
		fail(http.StatusBadGateway, fmt.Errorf("帖子中未提取到可转存的 123 云盘链接"))
		return
	}
	share := thread.Shares[0]
	task.ShareURL = share.URL
	task.Progress = 30
	_ = s.store.UpdateTask(task)

	// 2) 网盘侧：统一归档到影视合集/{韩剧、国产剧、美剧、电影、综艺...}
	panClient := s.panClient()
	if !panClient.Configured() {
		fail(http.StatusPreconditionFailed, fmt.Errorf(
			"论坛侧已解锁（分享链接已提取），但未配置 123云盘 token：执行 --pan123-token 换取后填入 .env 的 PAN123_TOKEN 并重启"))
		return
	}
	shareKey := ShareKeyOf(share.URL)
	if shareKey == "" {
		fail(http.StatusBadGateway, fmt.Errorf("无法从链接提取 shareKey: %s", share.URL))
		return
	}
	files, err := panClient.ShareInfo(shareKey, share.Code)
	if err != nil {
		fail(http.StatusBadGateway, err)
		return
	}
	task.Progress = 55
	task.FileCount = len(files)
	for _, file := range files {
		task.FileIDs = append(task.FileIDs, strconv.FormatInt(file.FileID, 10))
		task.TotalSize += file.Size
	}
	_ = s.store.UpdateTask(task)
	folderID, err := panClient.MkdirP(targetPath)
	if err != nil {
		fail(http.StatusBadGateway, fmt.Errorf("目标目录创建失败: %w", err))
		return
	}
	task.Progress = 75
	_ = s.store.UpdateTask(task)
	n, err := panClient.CopySave(shareKey, share.Code, folderID, files)
	if err != nil {
		fail(http.StatusBadGateway, err)
		return
	}
	for _, file := range files {
		task.Transferred = append(task.Transferred, model.File{
			ID: strconv.FormatInt(file.FileID, 10), Name: file.Filename,
			Size: file.Size, IsDir: file.IsDir(),
		})
	}
	task.Status = model.TaskStatusSuccess
	task.Progress = 100
	task.Error = ""
	task.FinishedAt = time.Now()
	_ = s.store.UpdateTask(task)
	s.invalidateDirSizes() // 新文件入库，父级目录大小缓存失效
	writeOK(w, map[string]any{
		"task_id": task.ID, "tid": req.TID, "title": req.Title,
		"already_unlocked": thread.AlreadyUnlocked,
		"share_url":        share.URL, "share_code": share.Code,
		"target_path": targetPath, "saved_files": n,
	})
}

// ShareKeyOf / PlanTargetPath 转发到 pan123web（供 handler 简用）。
var ShareKeyOf = pan123web.ShareKeyOf
var PlanTargetPath = pan123web.PlanTargetPath
