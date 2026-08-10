// Package api implements the HTTP handlers for the dashboard. Routes and
// JSON shapes match the Angular frontend (src/frontend) exactly; only the
// bearer-token wire format (see internal/auth) is independent of it -- fine
// since the frontend/backend are always redeployed together, atomically.
package api

import (
	"net/http"

	"wireguard-pro/internal/auth"
	"wireguard-pro/internal/config"
	"wireguard-pro/internal/db"
	"wireguard-pro/internal/wgmgr"
)

// Server holds the dependencies HTTP handlers need.
type Server struct {
	Config      *config.Config
	DB          *db.DB
	WGMgr       *wgmgr.Client
	Tokens      *auth.Tokens
	WGPublicKey string
}

// Routes registers all /api handlers onto mux.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("GET /api/config", s.requireAuth(s.handleGetConfig))
	mux.HandleFunc("POST /api/peers/new", s.requireAuth(s.handleCreatePeer))
	mux.HandleFunc("POST /api/peers/delete", s.requireAuth(s.handleDeletePeer))
	mux.HandleFunc("GET /api/peers/list", s.requireAuth(s.handleListPeers))
	mux.HandleFunc("GET /api/peers/stats", s.requireAuth(s.handlePeerStats))
	mux.HandleFunc("GET /api/serverinfo", s.requireAuth(s.handleServerInfo))
}
