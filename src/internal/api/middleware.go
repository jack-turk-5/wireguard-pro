package api

import (
	"errors"
	"net/http"
	"strings"

	"wireguard-pro/internal/auth"
)

// requireAuth extracts a Bearer token from the Authorization header and
// rejects the request if it's missing, malformed, or expired.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "Not authenticated")
			return
		}

		if _, err := s.Tokens.Verify(token); err != nil {
			msg := "Invalid token"
			if errors.Is(err, auth.ErrExpired) {
				msg = "Token has expired"
			}
			writeError(w, http.StatusUnauthorized, msg)
			return
		}

		next(w, r)
	}
}
