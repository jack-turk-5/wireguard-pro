package api

import "net/http"

// handleLogin verifies username/password from a form body and, on success,
// responds with a bearer token (POST /api/login).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid form body")
		return
	}
	username := r.PostFormValue("username")
	password := r.PostFormValue("password")

	ok, err := s.DB.VerifyUser(username, password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "Incorrect username or password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"access_token": s.Tokens.Generate(username),
		"token_type":   "bearer",
	})
}
