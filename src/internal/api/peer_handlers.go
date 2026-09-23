package api

import (
	"encoding/json"
	"net/http"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"wireguard-pro/internal/db"
	"wireguard-pro/internal/wgmgr"
)

// peerTimeLayout is the format peer created_at/expires_at timestamps are
// stored/rendered in -- matches the original app's
// strftime("%Y-%m-%d %H:%M:%S") exactly, so existing DB rows read back
// unchanged.
const peerTimeLayout = "2006-01-02 15:04:05"

type peerResponse struct {
	PublicKey   string `json:"public_key"`
	PrivateKey  string `json:"private_key"`
	Nickname    string `json:"nickname"`
	IPv4Address string `json:"ipv4_address"`
	IPv6Address string `json:"ipv6_address"`
	ExpiresAt   string `json:"expires_at"`
	CreatedAt   string `json:"created_at"`
}

// peerToResponse converts a db.Peer row to its JSON response shape.
func peerToResponse(p db.Peer) peerResponse {
	return peerResponse{
		PublicKey:   p.PublicKey,
		PrivateKey:  p.PrivateKey,
		Nickname:    p.Nickname,
		IPv4Address: p.IPv4Address,
		IPv6Address: p.IPv6Address,
		ExpiresAt:   p.ExpiresAt,
		CreatedAt:   p.CreatedAt,
	}
}

type createPeerRequest struct {
	DaysValid int `json:"days_valid"`
}

// handleCreatePeer allocates a new peer (key pair + next-available
// addresses), persists it, and configures it on the running device
// (POST /api/peers/new).
func (s *Server) handleCreatePeer(w http.ResponseWriter, r *http.Request) {
	req := createPeerRequest{DaysValid: 7}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	if req.DaysValid <= 0 {
		req.DaysValid = 7
	}

	existing, err := s.DB.ListPeers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	usedV4 := make(map[string]bool, len(existing))
	usedV6 := make(map[string]bool, len(existing))
	for _, p := range existing {
		usedV4[p.IPv4Address] = true
		usedV6[p.IPv6Address] = true
	}

	ipv4, ipv6, err := wgmgr.NextAvailableIP(s.Config.WGIPv4BaseAddr, s.Config.WGIPv6BaseAddr, usedV4, usedV6)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	priv, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pub := priv.PublicKey()

	now := time.Now().UTC()
	expiresAt := now.AddDate(0, 0, req.DaysValid).Format(peerTimeLayout)
	createdAt := now.Format(peerTimeLayout)

	if err := s.DB.AddPeer(pub.String(), priv.String(), ipv4, ipv6, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.WGMgr.AddPeer(pub, ipv4, ipv6); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, peerResponse{
		PublicKey:   pub.String(),
		PrivateKey:  priv.String(),
		IPv4Address: ipv4,
		IPv6Address: ipv6,
		ExpiresAt:   expiresAt,
		CreatedAt:   createdAt,
	})
}

type deletePeerRequest struct {
	PublicKey string `json:"public_key"`
}

// handleDeletePeer removes a peer from the DB and, if it existed, from the
// running device (POST /api/peers/delete).
func (s *Server) handleDeletePeer(w http.ResponseWriter, r *http.Request) {
	var req deletePeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deleted, err := s.DB.RemovePeer(req.PublicKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deleted {
		pub, err := wgtypes.ParseKey(req.PublicKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := s.WGMgr.RemovePeer(pub); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"deleted": deleted})
}

type renamePeerRequest struct {
	PublicKey string `json:"public_key"`
	Nickname  string `json:"nickname"`
}

// handleRenamePeer sets a peer's nickname (POST /api/peers/rename).
func (s *Server) handleRenamePeer(w http.ResponseWriter, r *http.Request) {
	var req renamePeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	renamed, err := s.DB.RenamePeer(req.PublicKey, req.Nickname)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !renamed {
		writeError(w, http.StatusNotFound, "peer not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"nickname": req.Nickname})
}

// handleListPeers returns every stored peer (GET /api/peers/list).
func (s *Server) handleListPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := s.DB.ListPeers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]peerResponse, len(peers))
	for i, p := range peers {
		resp[i] = peerToResponse(p)
	}
	writeJSON(w, http.StatusOK, resp)
}

// peerStatResponse matches the frontend's Stat interface (api.service.ts)
// exactly: public_key, last_handshake_time, rx_bytes, tx_bytes as JSON
// numbers. The original app sent these as strings straight from `wg show
// dump` output despite the frontend declaring them numeric -- an existing
// type mismatch that happened to not matter since nothing parsed them
// numerically; sending real numbers here is strictly more correct.
type peerStatResponse struct {
	PublicKey         string `json:"public_key"`
	LastHandshakeTime int64  `json:"last_handshake_time"`
	RxBytes           int64  `json:"rx_bytes"`
	TxBytes           int64  `json:"tx_bytes"`
}

// handlePeerStats returns live per-peer traffic/handshake stats
// (GET /api/peers/stats).
func (s *Server) handlePeerStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.WGMgr.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := make([]peerStatResponse, len(stats))
	for i, st := range stats {
		var lastHandshake int64
		if !st.LastHandshakeTime.IsZero() {
			lastHandshake = st.LastHandshakeTime.Unix()
		}
		resp[i] = peerStatResponse{
			PublicKey:         st.PublicKey.String(),
			LastHandshakeTime: lastHandshake,
			RxBytes:           st.ReceiveBytes,
			TxBytes:           st.TransmitBytes,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
