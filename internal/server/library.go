package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"resource-center/internal/model"
	"resource-center/internal/r2"
	"resource-center/internal/store"
	"resource-center/internal/zlib"
)

// ---- 模式 / 搜索 / 详情 ----

// handleLibStatus 书库当前模式（jump/direct）与跳转基址。
func (s *Server) handleLibStatus(w http.ResponseWriter, _ *http.Request) {
	mode, base := "jump", ""
	provider := s.bookProvider()
	if j, ok := provider.(*zlib.Jump); ok {
		mode, base = "jump", j.BaseURL
	} else {
		mode = "direct"
	}
	writeOK(w, map[string]string{"mode": mode, "base_url": base})
}

func (s *Server) handleLibSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	res, err := s.bookProvider().Search(r.Context(), model.BookQuery{
		Keyword: strings.TrimSpace(q.Get("q")),
		Page:    page,
		Lang:    q.Get("lang"),
		Ext:     q.Get("ext"),
		Sort:    q.Get("sort"),
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, res)
}

func (s *Server) handleLibDetail(w http.ResponseWriter, r *http.Request) {
	b, err := s.bookProvider().Detail(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeOK(w, b)
}

// ---- 书架 ----

type shelfReq struct {
	BookID   string   `json:"book_id"`
	Title    string   `json:"title"`
	Authors  []string `json:"authors"`
	Cover    string   `json:"cover"`
	Format   string   `json:"format"`
	SizeText string   `json:"size_text"`
}

func (s *Server) handleLibShelfList(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListShelf(currentUser(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, items)
}

func (s *Server) handleLibShelfSave(w http.ResponseWriter, r *http.Request) {
	var req shelfReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.BookID == "" || req.Title == "" {
		writeErr(w, http.StatusBadRequest, "book_id 和 title 不能为空")
		return
	}
	it := &model.ShelfItem{
		ID:       "shelf_" + newID(req.BookID),
		UserID:   currentUser(r),
		BookID:   req.BookID,
		Title:    req.Title,
		Authors:  req.Authors,
		Cover:    req.Cover,
		Format:   req.Format,
		SizeText: req.SizeText,
		AddedAt:  time.Now(),
	}
	if err := s.store.SaveShelfItem(it); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, it)
}

func (s *Server) handleLibShelfDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteShelfItem(currentUser(r), r.PathValue("bookID")); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "未收藏该书")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, nil)
}

// ---- 转存（下载 → R2） ----

type libTransferReq struct {
	BookID  string   `json:"book_id"`
	Title   string   `json:"title"`
	Authors []string `json:"authors"`
	Format  string   `json:"format"`
}

var unsafeKey = regexp.MustCompile(`[^\w.\-\x{4e00}-\x{9fa5}]+`)

// handleLibTransfer 创建转存任务：书籍文件流 → Cloudflare R2。
func (s *Server) handleLibTransfer(w http.ResponseWriter, r *http.Request) {
	var req libTransferReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.BookID == "" || req.Title == "" {
		writeErr(w, http.StatusBadRequest, "book_id 和 title 不能为空")
		return
	}
	destination := s.objectStore() // 提交时固定目标，避免转存中途更改默认 Bucket 造成错位。
	rec := &model.BookRecord{
		ID:          newID("book"),
		UserID:      currentUser(r),
		BookID:      req.BookID,
		Title:       req.Title,
		Authors:     req.Authors,
		Format:      strings.ToLower(req.Format),
		Status:      model.BookStatusQueued,
		StorageKind: destination.Kind(),
		CreatedAt:   time.Now(),
	}
	if r2Store, ok := destination.(r2ObjectStore); ok {
		rec.StorageBucket = r2Store.client.BucketName()
	}
	if err := s.store.CreateBookRecord(rec); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	go s.runBookTransfer(rec, destination)
	writeOK(w, rec)
}

// runBookTransfer 执行转存：详情兜底 → 下载流 → R2 PutObject。
func (s *Server) runBookTransfer(rec *model.BookRecord, destination ObjectStore) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	rec.Status = model.BookStatusSaving
	_ = s.store.UpdateBookRecord(rec)

	fail := func(msg string) {
		rec.Status = model.BookStatusFailed
		rec.Error = msg
		rec.FinishedAt = time.Now()
		_ = s.store.UpdateBookRecord(rec)
	}

	rc, size, format, err := s.bookProvider().Download(ctx, rec.BookID)
	if err != nil {
		fail(fmt.Sprintf("下载失败: %v", err))
		return
	}
	defer rc.Close()
	if rec.Format == "" {
		rec.Format = format
	}
	if rec.Format == "" {
		rec.Format = "bin"
	}

	// R2 键：books/{userID}/{bookID}.{ext}
	key := fmt.Sprintf("books/%s/%s.%s", rec.UserID, rec.BookID, rec.Format)
	put, err := destination.PutObject(key, rc, contentTypeOf(rec.Format), size)
	if err != nil {
		// R2 未配置时直接失败并给出明确指引
		fail(fmt.Sprintf("转存到对象存储失败: %v", err))
		return
	}
	rec.SizeBytes = put
	if size > 0 {
		rec.SizeBytes = size
	}
	rec.R2Key = key
	rec.Status = model.BookStatusDone
	rec.FinishedAt = time.Now()
	_ = s.store.UpdateBookRecord(rec)
}

func contentTypeOf(ext string) string {
	switch ext {
	case "pdf":
		return "application/pdf"
	case "epub":
		return "application/epub+zip"
	case "mobi", "azw3":
		return "application/x-mobipocket-ebook"
	default:
		return "application/octet-stream"
	}
}

// ---- 转存记录 ----

func (s *Server) handleLibHistory(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	list, total, err := s.store.ListBookRecords(currentUser(r), page, 20)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]any{"list": list, "total": total, "page": page})
}

func (s *Server) handleLibHistoryDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteBookRecord(currentUser(r), r.PathValue("id")); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "记录不存在")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, nil)
}

// handleLibDownload 从 R2 代理下载已转存的书籍。
func (s *Server) handleLibDownload(w http.ResponseWriter, r *http.Request) {
	rec, err := s.store.GetBookRecord(currentUser(r), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "记录不存在")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rec.Status != model.BookStatusDone || rec.R2Key == "" {
		writeErr(w, http.StatusConflict, "该记录尚未转存完成")
		return
	}

	cfg := s.configSnapshot()
	destination := s.objectStore()
	if rec.StorageKind == "local" {
		destination = NewLocalStore(cfg.DataDir)
	} else if rec.StorageKind == "r2" {
		accountID := cfg.CloudflareAccountID()
		destination = r2ObjectStore{client: r2.New(accountID, cfg.R2AccessKeyID, cfg.R2SecretAccessKey, rec.StorageBucket)}
	}
	// 公开地址只适用于当前默认 Bucket；历史 Bucket 使用 R2 API 代理下载。
	if cfg.R2PublicBase != "" && rec.StorageKind != "local" && (rec.StorageBucket == "" || rec.StorageBucket == cfg.R2Bucket) {
		u := strings.TrimRight(cfg.R2PublicBase, "/") + "/" + rec.R2Key
		http.Redirect(w, r, u, http.StatusFound)
		return
	}

	rc, size, ct, err := destination.GetObject(rec.R2Key)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer rc.Close()
	filename := url.PathEscape(sanitizeFilename(rec.Title) + "." + rec.Format)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+filename)
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	_, _ = io.Copy(w, rc)
}

func sanitizeFilename(name string) string {
	name = unsafeKey.ReplaceAllString(name, "_")
	return strings.Trim(name, "_.")
}
