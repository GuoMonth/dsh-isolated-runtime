package gateway

import (
	"encoding/json"
	"net/http"
)

// Authenticator establishes the trusted Principal from the transport request.
// A nil Authenticator denies every admission request.
type Authenticator interface {
	Authenticate(r *http.Request) (Principal, error)
}

// Server exposes the current internal admission primitive over HTTP.
//
// M2 grows this boundary into the authenticated runtime router/reverse proxy for
// DSH HTTP, WebSocket, and streaming traffic. Clients must never receive Pod,
// Service, Namespace, Node, or other topology details.
type Server struct {
	auth     Authenticator
	admitter *Admitter
}

// NewServer builds an HTTP server around an Authenticator and Admitter.
func NewServer(auth Authenticator, admitter *Admitter) *Server {
	return &Server{auth: auth, admitter: admitter}
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

	if s.auth == nil || s.admitter == nil {
		writeDecision(w, http.StatusForbidden, AdmissionDecision{Allowed: false})
		return
	}
	principal, err := s.auth.Authenticate(r)
	if err != nil || principal.Subject == "" {
		writeDecision(w, http.StatusForbidden, AdmissionDecision{Allowed: false})
		return
	}

	ctx := WithPrincipal(r.Context(), principal)
	decision, err := s.admitter.Admit(ctx, req)
	if err != nil {
		writeDecision(w, http.StatusForbidden, AdmissionDecision{Allowed: false})
		return
	}
	writeDecision(w, http.StatusOK, decision)
}

func writeDecision(w http.ResponseWriter, status int, decision AdmissionDecision) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(decision)
}
