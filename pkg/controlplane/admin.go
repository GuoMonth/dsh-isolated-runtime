package controlplane

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/GuoMonth/dsh-isolated-runtime/pkg/runtime"
)

const (
	AdminUsername = "Admin"
	AdminPassword = "Admin"
	adminCookie   = "dsh_admin_session"
)

type AdminServer struct {
	rt           runtime.Runtime
	sessionToken string
	mux          *http.ServeMux
}

func NewAdminServer(rt runtime.Runtime) (*AdminServer, error) {
	if rt == nil {
		return nil, errors.New("controlplane: runtime backend is required")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("controlplane: session secret: %w", err)
	}
	s := &AdminServer{rt: rt, sessionToken: base64.RawURLEncoding.EncodeToString(secret), mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

func (s *AdminServer) Handler() http.Handler { return s.mux }

func (s *AdminServer) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /login", s.handleLoginPage)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.uiAuth(s.handleLogout))
	s.mux.HandleFunc("GET /", s.uiAuth(s.handleDashboard))
	s.mux.HandleFunc("GET /runtimes", s.uiAuth(s.handleRuntimeList))
	s.mux.HandleFunc("GET /runtimes/new", s.uiAuth(s.handleRuntimeNew))
	s.mux.HandleFunc("POST /runtimes", s.uiAuth(s.handleRuntimeCreate))
	s.mux.HandleFunc("GET /runtimes/{name}", s.uiAuth(s.handleRuntimeDetail))
	s.mux.HandleFunc("POST /runtimes/{name}/delete", s.uiAuth(s.handleRuntimeDelete))
	s.mux.HandleFunc("GET /api/v1/runtimes", s.apiAuth(s.handleAPIList))
	s.mux.HandleFunc("POST /api/v1/runtimes", s.apiAuth(s.handleAPICreate))
	s.mux.HandleFunc("GET /api/v1/runtimes/{name}", s.apiAuth(s.handleAPIGet))
	s.mux.HandleFunc("DELETE /api/v1/runtimes/{name}", s.apiAuth(s.handleAPIDelete))
}

func (s *AdminServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *AdminServer) validCredentials(user, pass string) bool {
	return subtle.ConstantTimeCompare([]byte(user), []byte(AdminUsername)) == 1 && subtle.ConstantTimeCompare([]byte(pass), []byte(AdminPassword)) == 1
}

func (s *AdminServer) uiAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookie)
	return err == nil && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.sessionToken)) == 1
}

func (s *AdminServer) uiAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.uiAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (s *AdminServer) apiAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || !s.validCredentials(user, pass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="dsh-isolated-control-plane"`)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *AdminServer) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.uiAuthenticated(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "Login", loginTemplate, nil)
}

func (s *AdminServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !s.validCredentials(r.FormValue("username"), r.FormValue("password")) {
		s.renderStatus(w, http.StatusUnauthorized, "Login", loginTemplate, map[string]any{"Error": "Invalid username or password"})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: s.sessionToken, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: int((8 * time.Hour).Seconds())})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *AdminServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: adminCookie, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *AdminServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	items, err := s.rt.List(r.Context(), "")
	if err != nil {
		s.renderError(w, err)
		return
	}
	counts := map[string]int{"Total": len(items)}
	for _, item := range items {
		counts[item.Phase]++
	}
	s.render(w, "Dashboard", dashboardTemplate, counts)
}

func (s *AdminServer) handleRuntimeList(w http.ResponseWriter, r *http.Request) {
	items, err := s.rt.List(r.Context(), "")
	if err != nil {
		s.renderError(w, err)
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	s.render(w, "Runtimes", runtimeListTemplate, items)
}

func (s *AdminServer) handleRuntimeNew(w http.ResponseWriter, _ *http.Request) {
	s.render(w, "New Runtime", runtimeNewTemplate, nil)
}

func (s *AdminServer) handleRuntimeCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	spec := runtime.Spec{
		Name:             strings.TrimSpace(r.FormValue("name")),
		Tenant:           strings.TrimSpace(r.FormValue("tenant")),
		Image:            strings.TrimSpace(r.FormValue("image")),
		RuntimeClass:     strings.TrimSpace(r.FormValue("runtimeClass")),
		SecurityClass:    runtime.SecurityClass(r.FormValue("securityClass")),
		NetworkIsolation: r.FormValue("networkIsolation") == "on",
		ResourceLimits:   resourceLimits(r.FormValue("cpu"), r.FormValue("memory")),
	}
	if _, err := s.rt.Create(r.Context(), spec); err != nil {
		s.renderStatus(w, http.StatusBadRequest, "New Runtime", runtimeNewTemplate, map[string]any{"Error": err.Error(), "Spec": spec})
		return
	}
	http.Redirect(w, r, "/runtimes/"+spec.Name+"?tenant="+spec.Tenant, http.StatusSeeOther)
}

func (s *AdminServer) handleRuntimeDetail(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	info, err := s.rt.Get(r.Context(), tenant, r.PathValue("name"))
	if err != nil {
		s.renderErrorStatus(w, err)
		return
	}
	s.render(w, "Runtime "+info.Name, runtimeDetailTemplate, info)
}

func (s *AdminServer) handleRuntimeDelete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.rt.Delete(r.Context(), r.FormValue("tenant"), r.PathValue("name")); err != nil {
		s.renderErrorStatus(w, err)
		return
	}
	http.Redirect(w, r, "/runtimes", http.StatusSeeOther)
}

func (s *AdminServer) handleAPIList(w http.ResponseWriter, r *http.Request) {
	items, err := s.rt.List(r.Context(), r.URL.Query().Get("tenant"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *AdminServer) handleAPICreate(w http.ResponseWriter, r *http.Request) {
	var spec runtime.Spec
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&spec); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	info, err := s.rt.Create(r.Context(), spec)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, runtime.ErrConflict) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *AdminServer) handleAPIGet(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	if tenant == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant is required"})
		return
	}
	info, err := s.rt.Get(r.Context(), tenant, r.PathValue("name"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, runtime.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *AdminServer) handleAPIDelete(w http.ResponseWriter, r *http.Request) {
	tenant := r.URL.Query().Get("tenant")
	if tenant == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tenant is required"})
		return
	}
	if err := s.rt.Delete(r.Context(), tenant, r.PathValue("name")); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, runtime.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func resourceLimits(cpu, memory string) map[string]string {
	limits := map[string]string{}
	if v := strings.TrimSpace(cpu); v != "" {
		limits["cpu"] = v
	}
	if v := strings.TrimSpace(memory); v != "" {
		limits["memory"] = v
	}
	if len(limits) == 0 {
		return nil
	}
	return limits
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *AdminServer) renderError(w http.ResponseWriter, err error) {
	s.renderStatus(w, http.StatusInternalServerError, "Error", errorTemplate, err.Error())
}

func (s *AdminServer) renderErrorStatus(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, runtime.ErrNotFound) {
		status = http.StatusNotFound
	}
	s.renderStatus(w, status, "Error", errorTemplate, err.Error())
}

func (s *AdminServer) render(w http.ResponseWriter, title, body string, data any) {
	s.renderStatus(w, http.StatusOK, title, body, data)
}

func (s *AdminServer) renderStatus(w http.ResponseWriter, status int, title, body string, data any) {
	page, err := template.New("page").Parse(layoutTemplate + body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = page.ExecuteTemplate(w, "layout", map[string]any{"Title": title, "Data": data})
}

const layoutTemplate = `{{define "layout"}}<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}} · DSH Isolated</title><style>body{font-family:ui-sans-serif,system-ui,-apple-system,sans-serif;margin:0;background:#f5f5f5;color:#18181b}nav{background:#18181b;color:white;padding:14px 24px;display:flex;gap:20px;align-items:center}nav a{color:white;text-decoration:none}main{max-width:1100px;margin:28px auto;padding:0 20px}.card{background:white;border:1px solid #e4e4e7;border-radius:10px;padding:20px;margin-bottom:18px}table{width:100%;border-collapse:collapse}th,td{text-align:left;border-bottom:1px solid #e4e4e7;padding:10px}input,select{width:100%;box-sizing:border-box;padding:9px;margin:5px 0 14px;border:1px solid #d4d4d8;border-radius:6px}button,.button{display:inline-block;background:#18181b;color:white;border:0;border-radius:6px;padding:9px 14px;text-decoration:none;cursor:pointer}.danger{background:#b91c1c}.muted{color:#71717a}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px}.metric{font-size:28px;font-weight:700}.error{color:#b91c1c;background:#fef2f2;padding:10px;border-radius:6px}code{background:#f4f4f5;padding:2px 5px;border-radius:4px}</style></head><body><nav><strong>DSH Isolated Control Plane</strong><a href="/">Dashboard</a><a href="/runtimes">Runtimes</a><span style="margin-left:auto"><form method="post" action="/logout"><button>Logout</button></form></span></nav><main>{{template "content" .Data}}</main></body></html>{{end}}`
const loginTemplate = `{{define "content"}}<div class="card" style="max-width:420px;margin:80px auto"><h1>Admin login</h1><p class="muted">M1 bootstrap authentication. Default credentials are <code>Admin / Admin</code>.</p>{{if .Error}}<p class="error">{{.Error}}</p>{{end}}<form method="post" action="/login"><label>Username</label><input name="username" value="Admin" autocomplete="username"><label>Password</label><input name="password" type="password" value="Admin" autocomplete="current-password"><button type="submit">Sign in</button></form></div>{{end}}`
const dashboardTemplate = `{{define "content"}}<h1>Dashboard</h1><div class="grid"><div class="card"><div class="muted">Total</div><div class="metric">{{index . "Total"}}</div></div><div class="card"><div class="muted">Running</div><div class="metric">{{index . "Running"}}</div></div><div class="card"><div class="muted">Pending</div><div class="metric">{{index . "Pending"}}</div></div><div class="card"><div class="muted">Failed</div><div class="metric">{{index . "Failed"}}</div></div></div><p><a class="button" href="/runtimes/new">Create Runtime</a></p>{{end}}`
const runtimeListTemplate = `{{define "content"}}<h1>Runtimes</h1><p><a class="button" href="/runtimes/new">Create Runtime</a></p><div class="card"><table><thead><tr><th>Name</th><th>Tenant</th><th>Image</th><th>Security</th><th>Phase</th></tr></thead><tbody>{{range .}}<tr><td><a href="/runtimes/{{.Name}}?tenant={{.Tenant}}">{{.Name}}</a></td><td>{{.Tenant}}</td><td>{{.Image}}</td><td>{{.SecurityClass}}</td><td>{{.Phase}}</td></tr>{{else}}<tr><td colspan="5" class="muted">No runtimes</td></tr>{{end}}</tbody></table></div>{{end}}`
const runtimeNewTemplate = `{{define "content"}}<h1>New Runtime</h1><div class="card">{{if .Error}}<p class="error">{{.Error}}</p>{{end}}<form method="post" action="/runtimes"><label>Name</label><input name="name" required placeholder="demo"><label>Tenant</label><input name="tenant" required placeholder="tenant-a"><label>Image</label><input name="image" required placeholder="nginxinc/nginx-unprivileged:alpine"><label>Security class</label><select name="securityClass"><option value="standard">standard</option><option value="sandboxed">sandboxed</option></select><label>RuntimeClass (required for sandboxed)</label><input name="runtimeClass" placeholder="gvisor"><label>CPU limit</label><input name="cpu" placeholder="500m"><label>Memory limit</label><input name="memory" placeholder="512Mi"><label><input style="width:auto" type="checkbox" name="networkIsolation" checked> deny-all network isolation</label><br><br><button type="submit">Create Runtime</button></form></div>{{end}}`
const runtimeDetailTemplate = `{{define "content"}}<h1>{{.Name}}</h1><div class="card"><p><strong>Tenant:</strong> {{.Tenant}}</p><p><strong>Image:</strong> {{.Image}}</p><p><strong>Security:</strong> {{.SecurityClass}}{{if .RuntimeClass}} / {{.RuntimeClass}}{{end}}</p><p><strong>Phase:</strong> {{.Phase}}</p><p><strong>Address:</strong> {{.Address}}</p>{{if .Message}}<p><strong>Message:</strong> {{.Message}}</p>{{end}}<form method="post" action="/runtimes/{{.Name}}/delete"><input type="hidden" name="tenant" value="{{.Tenant}}"><button class="danger" type="submit">Delete Runtime</button></form></div>{{end}}`
const errorTemplate = `{{define "content"}}<h1>Error</h1><div class="card error">{{.}}</div>{{end}}`
