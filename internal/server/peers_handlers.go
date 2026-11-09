package server

import (
	"net/http"

	"github.com/lupguo/landrop/internal/hub"
)

// handlePeers: GET /api/peers.
func (s *server) handlePeers(w http.ResponseWriter, r *http.Request) {
	peers := s.Hub.Peers()
	if peers == nil {
		peers = []hub.Peer{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"peers": peers})
}

// handleKick: DELETE /api/peers/{peer_id} (host only).
func (s *server) handleKick(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("peer_id")
	if id == sessionFrom(r).PeerID {
		writeErr(w, http.StatusBadRequest, "self")
		return
	}
	s.Auth.RevokePeer(id)
	s.Hub.Kick(id)
	w.WriteHeader(http.StatusNoContent)
}
