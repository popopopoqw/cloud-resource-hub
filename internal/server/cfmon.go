package server

import (
	"net/http"
)

// GET /api/v1/cfmon/status
func (s *Server) handleCfmonStatus(w http.ResponseWriter, _ *http.Request) {
	if s.monitor() == nil {
		writeErr(w, http.StatusBadRequest, "Cloudflare 监控未配置：请到设置中心配置 API Token 与 Account ID")
		return
	}
	writeOK(w, s.monitor().Status())
}

// GET /api/v1/cfmon/overview
func (s *Server) handleCfmonOverview(w http.ResponseWriter, r *http.Request) {
	if s.monitor() == nil {
		writeErr(w, http.StatusBadRequest, "Cloudflare 监控未配置：请到设置中心配置 API Token 与 Account ID")
		return
	}
	o, err := s.monitor().Overview(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, o)
}
