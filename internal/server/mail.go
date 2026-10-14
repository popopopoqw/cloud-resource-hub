package server

import (
	"io"
	"net/http"
	"net/url"
	"strconv"

	"resource-center/internal/mail"
)

// mailProv 取路径中的邮箱供应商；未注册时 404。
func (s *Server) mailProv(r *http.Request) (mail.Provider, bool) {
	p, ok := s.mailRegistry().Get(r.PathValue("prov"))
	return p, ok
}

// GET /api/v1/mail/providers 供应商列表（前端切换 tab 用）
func (s *Server) handleMailProviders(w http.ResponseWriter, _ *http.Request) {
	type item struct {
		ID     string         `json:"id"`
		Name   string         `json:"name"`
		Status map[string]any `json:"status"`
	}
	out := []item{}
	registry := s.mailRegistry()
	// Gmail 是已上线的主邮箱供应商：即使尚未授权，也要固定展示，
	// 由前端给出配置引导，避免列表只剩 Outlook 规划占位项。
	if p, ok := registry.Get("gmail"); ok {
		status := p.Status()
		if status == nil {
			status = map[string]any{}
		}
		status["configured"] = true
		out = append(out, item{ID: p.ID(), Name: p.Name(), Status: status})
	} else {
		out = append(out, item{ID: "gmail", Name: "Gmail", Status: map[string]any{
			"configured": false, "coming": false, "email": "", "scope": "gmail.readonly",
		}})
	}
	for _, id := range registry.IDs() {
		if id == "gmail" { // Gmail 已固定放在第一位
			continue
		}
		p, _ := registry.Get(id)
		status := p.Status()
		if status == nil {
			status = map[string]any{}
		}
		status["configured"] = true
		out = append(out, item{ID: p.ID(), Name: p.Name(), Status: status})
	}
	// 后期上线的供应商先占位展示
	for _, placeholder := range []struct{ id, name string }{{"outlook", "Outlook"}} {
		if _, ok := registry.Get(placeholder.id); !ok {
			out = append(out, item{ID: placeholder.id, Name: placeholder.name,
				Status: map[string]any{"mock": false, "coming": true, "email": "", "scope": "-"}})
		}
	}
	writeOK(w, out)
}

// GET /api/v1/mail/{prov}/status
func (s *Server) handleMailStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := s.mailProv(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "未接入该邮箱供应商")
		return
	}
	writeOK(w, p.Status())
}

// GET /api/v1/mail/{prov}/messages?q=&page_token=
func (s *Server) handleMailList(w http.ResponseWriter, r *http.Request) {
	p, ok := s.mailProv(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "未接入该邮箱供应商")
		return
	}
	list, err := p.List(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("page_token"))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, list)
}

// GET /api/v1/mail/{prov}/messages/{id}
func (s *Server) handleMailGet(w http.ResponseWriter, r *http.Request) {
	p, ok := s.mailProv(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "未接入该邮箱供应商")
		return
	}
	d, err := p.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeOK(w, d)
}

// GET /api/v1/mail/{prov}/attachment/{id}/{attID}
func (s *Server) handleMailAttachment(w http.ResponseWriter, r *http.Request) {
	p, ok := s.mailProv(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "未接入该邮箱供应商")
		return
	}
	rc, mime, size, err := p.AttachmentData(r.Context(), r.PathValue("id"), r.PathValue("attID"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", orDefault(mime, "application/octet-stream"))
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.PathEscape("attachment.bin"))
	_, _ = io.Copy(w, rc) // 浏览器取消下载导致的中断属正常，忽略
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
