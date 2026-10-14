package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"resource-center/internal/pan"
)

// driveFS 取「我的网盘」的文件管理实现：
// 配置了 123云盘凭据（PAN123_TOKEN/账密）→ pan123web 真实数据；
// 否则回落 Mock 演示树。
// invalidateDirSizes 写操作后失效目录大小缓存（真实模式才有效）。
func (s *Server) invalidateDirSizes() {
	if fs := s.driveFileSystem(); fs != nil {
		fs.InvalidateSizes()
	}
}

func (s *Server) driveFS() (pan.FS, error) {
	if fs := s.driveFileSystem(); fs != nil && fs.c.Configured() {
		return fs, nil // 单例：目录大小缓存跨请求生效
	}
	p, ok := s.reg.Get("123pan")
	if !ok {
		return nil, errors.New("网盘未启用")
	}
	fs, ok := p.(pan.FS)
	if !ok {
		return nil, errors.New("该网盘暂不支持文件管理")
	}
	return fs, nil
}

func pathID(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
	return v
}

// GET /api/v1/drive/list?dir=&page=
func (s *Server) handleDriveList(w http.ResponseWriter, r *http.Request) {
	fs, err := s.driveFS()
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err.Error())
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	listing, err := fs.ListDir(r.Context(), pathID(r, "dir"), page)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, listing)
}

// GET /api/v1/drive/tree?dir=
func (s *Server) handleDriveTree(w http.ResponseWriter, r *http.Request) {
	fs, err := s.driveFS()
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err.Error())
		return
	}
	tree, err := fs.DirTree(r.Context(), pathID(r, "dir"))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, tree)
}

// GET /api/v1/drive/dirsize/{file_id} 递归校验目录大小（诊断备用，前端默认不调用）。
func (s *Server) handleDriveDirSize(w http.ResponseWriter, r *http.Request) {
	fs, err := s.driveFS()
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err.Error())
		return
	}
	pfs, ok := fs.(*panWebFS)
	if !ok {
		writeOK(w, map[string]int64{"size": 0}) // Mock：目录大小无意义
		return
	}
	id, err := strconv.ParseInt(r.PathValue("file_id"), 10, 64)
	if err != nil || id < 0 {
		writeErr(w, http.StatusBadRequest, "file_id 非法")
		return
	}
	if r.URL.Query().Get("force") == "1" {
		pfs.InvalidateSizes() // 强制重算
	} else if size := pfs.DirSizeCached(id); size >= 0 {
		writeOK(w, map[string]int64{"size": size, "cached": 1})
		return
	}
	// 后台计算，本次先回 -1，前端稍后再取
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if _, err := pfs.DirSizeCompute(ctx, id); err != nil {
			log.Printf("[drive] 目录大小计算失败(%d): %v", id, err)
		}
	}()
	writeOK(w, map[string]int64{"size": -1, "computing": 1})
}

// GET /api/v1/drive/capacity
func (s *Server) handleDriveCapacity(w http.ResponseWriter, r *http.Request) {
	fs, err := s.driveFS()
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err.Error())
		return
	}
	used, total, err := fs.Capacity(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, map[string]int64{"used": used, "total": total})
}

type driveReq struct {
	ParentID int64   `json:"parent_id"`
	FileID   int64   `json:"file_id"`
	FileIDs  []int64 `json:"file_ids"`
	TargetID int64   `json:"target_id"`
	Name     string  `json:"name"`
}

// POST /api/v1/drive/mkdir
func (s *Server) handleDriveMkdir(w http.ResponseWriter, r *http.Request) {
	fs, err := s.driveFS()
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err.Error())
		return
	}
	var req driveReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeErr(w, http.StatusBadRequest, "文件夹名称不能为空")
		return
	}
	id, err := fs.Mkdir(r.Context(), req.ParentID, strings.TrimSpace(req.Name))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	s.invalidateDirSizes()
	writeOK(w, map[string]int64{"file_id": id})
}

// POST /api/v1/drive/rename
func (s *Server) handleDriveRename(w http.ResponseWriter, r *http.Request) {
	fs, err := s.driveFS()
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err.Error())
		return
	}
	var req driveReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.FileID == 0 || strings.TrimSpace(req.Name) == "" {
		writeErr(w, http.StatusBadRequest, "file_id 和 name 不能为空")
		return
	}
	if err := fs.Rename(r.Context(), req.FileID, strings.TrimSpace(req.Name)); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	s.invalidateDirSizes()
	writeOK(w, nil)
}

// POST /api/v1/drive/move
func (s *Server) handleDriveMove(w http.ResponseWriter, r *http.Request) {
	fs, err := s.driveFS()
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err.Error())
		return
	}
	var req driveReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.FileIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "file_ids 不能为空")
		return
	}
	if err := fs.Move(r.Context(), req.FileIDs, req.TargetID); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	s.invalidateDirSizes()
	writeOK(w, nil)
}

// POST /api/v1/drive/trash
func (s *Server) handleDriveTrash(w http.ResponseWriter, r *http.Request) {
	fs, err := s.driveFS()
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err.Error())
		return
	}
	var req driveReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.FileIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "file_ids 不能为空")
		return
	}
	if err := fs.Trash(r.Context(), req.FileIDs); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	s.invalidateDirSizes()
	writeOK(w, nil)
}

// GET /api/v1/drive/download/{file_id}
func (s *Server) handleDriveDownload(w http.ResponseWriter, r *http.Request) {
	fs, err := s.driveFS()
	if err != nil {
		writeErr(w, http.StatusNotImplemented, err.Error())
		return
	}
	id, err := strconv.ParseInt(r.PathValue("file_id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, "file_id 非法")
		return
	}
	u, err := fs.DownloadURL(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	http.Redirect(w, r, u, http.StatusFound)
}
