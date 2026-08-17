package gateway

import (
	"encoding/json"
	"net/http"
)

// Server exposes the Gateway over HTTP.
type Server struct {
	admitter *Admitter
}

// NewServer builds an HTTP server around an Admitter.
func NewServer(a *Admitter) *Server {
	return &Server{admitter: a}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/admit", s.handleAdmit)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// handleAdmit decodes an AdmissionRequest and returns the decision. Admission
// failure returns 403; the body always carries the reason.
func (s *Server) handleAdmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req AdmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	dec, err := s.admitter.Admit(r.Context(), req)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(dec)
		return
	}
	_ = json.NewEncoder(w).Encode(dec)
}
