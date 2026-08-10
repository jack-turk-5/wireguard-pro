package api

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError responds with {"detail": ...}, matching FastAPI's default
// HTTPException body shape closely enough for the frontend's error handling
// (internal/../frontend's jwt.interceptor.ts only branches on HTTP status,
// never inspects the body).
func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}
