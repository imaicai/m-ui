package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ServerConfig struct {
	Addr       string
	XrayBin    string
	XrayConfig string
}

type Server struct {
	store    *Store
	xray     *XrayManager
	cfg      ServerConfig
	sessions sync.Map
	statsMu   sync.Mutex
	trafficMu sync.Mutex
	lastNet   netSample
	startedAt time.Time
	loginMu  sync.Mutex
	failures map[string]*loginFailure
}

type loginFailure struct {
	Count int
	Until time.Time
}

type session struct {
	Expires time.Time
	CSRF    string
}

type PublicSettings struct {
	PanelName       string `json:"panel_name"`
	PanelPath       string `json:"panel_path"`
	PanelPort       int    `json:"panel_port"`
	Domain          string `json:"domain"`
	CertFile        string `json:"cert_file"`
	KeyFile         string `json:"key_file"`
	DefaultProtocol string `json:"default_protocol"`
	AdminUser       string `json:"admin_user"`
	CertExists      bool   `json:"cert_exists"`
	KeyExists       bool   `json:"key_exists"`
}

type NodeView struct {
	Node
	URI string `json:"uri"`
}

type Bootstrap struct {
	Version          string         `json:"version"`
	Author           string         `json:"author"`
	Settings         PublicSettings `json:"settings"`
	Nodes            []NodeView     `json:"nodes"`
	Subscriptions    []Subscription `json:"subscriptions"`
	RestartRequired  bool           `json:"restart_required,omitempty"`
	ReloginRequired  bool           `json:"relogin_required,omitempty"`
	PanelPathChanged bool           `json:"panel_path_changed,omitempty"`
}

func NewServer(store *Store, cfg ServerConfig) *Server {
	return &Server{store: store, xray: NewXrayManager(cfg.XrayBin, cfg.XrayConfig), cfg: cfg, startedAt: time.Now(), failures: map[string]*loginFailure{}}
}

func (s *Server) panelPath() string {
	p, err := normalizePanelPath(s.store.Snapshot().Settings.PanelPath)
	if err != nil || p == "" {
		return "/"
	}
	return p
}

const mUIServicePath = "/etc/systemd/system/m-ui.service"

func (s *Server) runningPanelPort() int {
	p, _ := strconv.Atoi(panelPort(s.cfg.Addr))
	if p < 1 {
		return 16800
	}
	return p
}

func configuredPanelPortFromService(content string, fallback int) int {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Environment=MUI_ADDR=") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "Environment=MUI_ADDR="))
		if p, err := strconv.Atoi(panelPort(v)); err == nil && p >= 1024 && p <= 65535 {
			return p
		}
	}
	return fallback
}

func (s *Server) configuredPanelPort() int {
	fallback := s.runningPanelPort()
	b, err := os.ReadFile(mUIServicePath)
	if err != nil {
		return fallback
	}
	return configuredPanelPortFromService(string(b), fallback)
}

func replaceServicePanelPort(content string, port int) (string, error) {
	if port < 1024 || port > 65535 {
		return "", errors.New("面板端口必须是 1024-65535")
	}
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Environment=MUI_ADDR=") {
			lines[i] = fmt.Sprintf("Environment=MUI_ADDR=0.0.0.0:%d", port)
			found = true
			break
		}
	}
	if !found {
		return "", errors.New("systemd 服务缺少 MUI_ADDR 配置")
	}
	return strings.Join(lines, "\n"), nil
}

func ensurePanelPortAvailable(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return fmt.Errorf("端口 %d 已被占用", port)
	}
	_ = ln.Close()
	return nil
}

func openLocalFirewallPort(port int) {
	if path, err := exec.LookPath("ufw"); err == nil {
		if out, err := exec.Command(path, "status").CombinedOutput(); err == nil && strings.Contains(strings.ToLower(string(out)), "status: active") {
			_ = exec.Command(path, "allow", fmt.Sprintf("%d/tcp", port)).Run()
			return
		}
	}
	if path, err := exec.LookPath("firewall-cmd"); err == nil {
		if err := exec.Command(path, "--state").Run(); err == nil {
			_ = exec.Command(path, "--permanent", "--add-port", fmt.Sprintf("%d/tcp", port)).Run()
			_ = exec.Command(path, "--reload").Run()
		}
	}
}

func (s *Server) stagePanelPort(port int) error {
	if port == s.configuredPanelPort() {
		return nil
	}
	if port != s.runningPanelPort() {
		if err := ensurePanelPortAvailable(port); err != nil {
			return err
		}
	}
	b, err := os.ReadFile(mUIServicePath)
	if err != nil {
		return fmt.Errorf("读取 m-ui systemd 服务失败: %w", err)
	}
	next, err := replaceServicePanelPort(string(b), port)
	if err != nil {
		return err
	}
	tmp := mUIServicePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(next), 0644); err != nil {
		return fmt.Errorf("写入端口配置失败: %w", err)
	}
	if err := os.Rename(tmp, mUIServicePath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存端口配置失败: %w", err)
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemd 重载失败: %s", strings.TrimSpace(string(out)))
	}
	openLocalFirewallPort(port)
	return nil
}

func (s *Server) Run() error {
	panelMux := http.NewServeMux()
	panelMux.HandleFunc("/api/panel-info", s.handlePanelInfo)
	panelMux.HandleFunc("/api/login", s.handleLogin)
	panelMux.HandleFunc("/api/logout", s.auth(s.handleLogout))
	panelMux.HandleFunc("/api/me", s.auth(s.handleMe))
	panelMux.HandleFunc("/api/bootstrap", s.auth(s.handleBootstrap))
	panelMux.HandleFunc("/api/system/stats", s.auth(s.handleStats))
	panelMux.HandleFunc("/api/system/restart", s.auth(s.handlePanelRestart))
	panelMux.HandleFunc("/api/settings", s.auth(s.handleSettings))
	panelMux.HandleFunc("/api/nodes/default", s.auth(s.handleCreateDefaultNode))
	panelMux.HandleFunc("/api/nodes/reset-traffic", s.auth(s.handleResetAllTraffic))
	panelMux.HandleFunc("/api/nodes/", s.auth(s.handleNode))
	panelMux.HandleFunc("/api/subscriptions", s.auth(s.handleSubscriptions))
	panelMux.HandleFunc("/api/subscriptions/", s.auth(s.handleSubscription))

	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(sub))
	panelMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if path == "." || path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/index.html"
		fileServer.ServeHTTP(w, r2)
	})

	rootMux := http.NewServeMux()
	rootMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	})
	rootMux.HandleFunc("/sub/", s.handleSubscriptionFeed)
	rootMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		base := s.panelPath()
		if base == "/" {
			panelMux.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == strings.TrimSuffix(base, "/") {
			http.Redirect(w, r, base, http.StatusTemporaryRedirect)
			return
		}
		if !strings.HasPrefix(r.URL.Path, base) {
			http.NotFound(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		rest := strings.TrimPrefix(r.URL.Path, base)
		r2.URL.Path = "/" + strings.TrimPrefix(rest, "/")
		if r2.URL.Path == "/" {
			r2.URL.Path = "/"
		}
		panelMux.ServeHTTP(w, r2)
	})

	handler := securityHeaders(s.redirectHTTPS(rootMux))
	go s.trafficLoop()
	return s.servePanel(handler)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func requestSameOrigin(r *http.Request) bool {
	if site := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")); site != "" && site != "same-origin" && site != "none" {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || !strings.EqualFold(u.Host, r.Host) {
		return false
	}
	if r.TLS != nil {
		return strings.EqualFold(u.Scheme, "https")
	}
	return strings.EqualFold(u.Scheme, "http")
}

func constantTokenEqual(a, b string) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("mui_session")
		if err != nil || c.Value == "" {
			writeError(w, 401, "请先登录")
			return
		}
		v, ok := s.sessions.Load(c.Value)
		if !ok || time.Now().After(v.(session).Expires) {
			if ok {
				s.sessions.Delete(c.Value)
			}
			writeError(w, 401, "登录状态已过期")
			return
		}
		sess := v.(session)
		if isMutatingMethod(r.Method) {
			if !requestSameOrigin(r) || !constantTokenEqual(strings.TrimSpace(r.Header.Get("X-MUI-CSRF")), sess.CSRF) {
				writeError(w, http.StatusForbidden, "安全校验失败，请刷新页面后重试")
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	ip := clientIP(r)
	if remain := s.loginBlocked(ip); remain > 0 {
		writeError(w, 429, fmt.Sprintf("登录失败次数过多，请 %d 分钟后再试", int(remain.Minutes())+1))
		return
	}
	var in struct{ Username, Password string }
	if !decodeJSON(w, r, &in) {
		return
	}
	st := s.store.Snapshot()
	if subtleUserCompare(strings.TrimSpace(in.Username), st.Settings.AdminUser) == false || !verifyPassword(in.Password, st.Settings.PasswordSalt, st.Settings.PasswordHash) {
		s.recordLoginFailure(ip)
		time.Sleep(250 * time.Millisecond)
		writeError(w, 401, "用户名或密码错误")
		return
	}
	s.clearLoginFailure(ip)
	if passwordNeedsUpgrade(st.Settings.PasswordHash) {
		_ = s.store.Update(func(next *AppState) error {
			return setPassword(next, in.Password)
		})
	}
	token := randomSessionToken()
	csrf := randomSessionToken()
	exp := time.Now().Add(7 * 24 * time.Hour)
	s.sessions.Store(token, session{Expires: exp, CSRF: csrf})
	cookiePath := s.panelPath()
	http.SetCookie(w, &http.Cookie{Name: "mui_session", Value: token, Path: cookiePath, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, Expires: exp, MaxAge: int((7 * 24 * time.Hour).Seconds())})
	http.SetCookie(w, &http.Cookie{Name: "mui_csrf", Value: csrf, Path: cookiePath, HttpOnly: false, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, Expires: exp, MaxAge: int((7 * 24 * time.Hour).Seconds())})
	writeJSON(w, 200, map[string]any{"ok": true, "username": st.Settings.AdminUser})
}

func subtleUserCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("mui_session"); err == nil {
		s.sessions.Delete(c.Value)
	}
	path := s.panelPath()
	http.SetCookie(w, &http.Cookie{Name: "mui_session", Value: "", Path: path, HttpOnly: true, Secure: r.TLS != nil, MaxAge: -1, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: "mui_csrf", Value: "", Path: path, HttpOnly: false, Secure: r.TLS != nil, MaxAge: -1, SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	st := s.store.Snapshot()
	writeJSON(w, 200, map[string]any{"username": st.Settings.AdminUser, "name": st.Settings.PanelName, "version": version, "author": "MiniJet"})
}

func (s *Server) handlePanelInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	st := s.store.Snapshot()
	writeJSON(w, 200, map[string]any{
		"name":       st.Settings.PanelName,
		"version":    version,
		"author":     "MiniJet",
		"panel_port": s.configuredPanelPort(),
	})
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	_ = s.syncTraffic()
	st := s.store.Snapshot()
	out := makeBootstrap(st)
	out.Settings.PanelPort = s.configuredPanelPort()
	writeJSON(w, 200, out)
}

func makeBootstrap(st AppState) Bootstrap {
	nodes := make([]NodeView, 0, len(st.Nodes))
	for _, n := range st.Nodes {
		nodes = append(nodes, NodeView{Node: n, URI: nodeURI(st, n)})
	}
	_, ce := os.Stat(st.Settings.CertFile)
	_, ke := os.Stat(st.Settings.KeyFile)
	return Bootstrap{
		Version: version, Author: "MiniJet",
		Settings: PublicSettings{
			PanelName: st.Settings.PanelName, PanelPath: st.Settings.PanelPath, Domain: st.Settings.Domain, CertFile: st.Settings.CertFile,
			KeyFile: st.Settings.KeyFile, DefaultProtocol: st.Settings.DefaultProtocol, AdminUser: st.Settings.AdminUser,
			CertExists: ce == nil, KeyExists: ke == nil,
		},
		Nodes: nodes, Subscriptions: st.Subscriptions,
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	s.statsMu.Lock()
	st, sample := readSystemStats(&s.lastNet)
	s.lastNet = sample
	s.statsMu.Unlock()
	if !s.startedAt.IsZero() {
		st.PanelUptimeSeconds = uint64(time.Since(s.startedAt).Seconds())
	}
	writeJSON(w, 200, st)
}

func (s *Server) handlePanelRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"message": "m-ui 正在重启",
	})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(700 * time.Millisecond)
		// The installer configures systemd with Restart=on-failure, so a controlled
		// non-zero exit restarts only the panel process without touching Xray.
		os.Exit(42)
	}()
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.handleBootstrap(w, r)
		return
	}
	if r.Method != http.MethodPut {
		methodNotAllowed(w)
		return
	}
	var in struct {
		PanelName       string `json:"panel_name"`
		PanelPath       string `json:"panel_path"`
		PanelPort       int    `json:"panel_port"`
		Domain          string `json:"domain"`
		CertFile        string `json:"cert_file"`
		KeyFile         string `json:"key_file"`
		AdminUser       string `json:"admin_user"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}

	oldConfiguredPort := s.configuredPanelPort()
	requestedPort := in.PanelPort
	if requestedPort == 0 {
		requestedPort = oldConfiguredPort
	}
	if requestedPort < 1024 || requestedPort > 65535 {
		writeError(w, 400, "面板端口必须是 1024-65535")
		return
	}

	old := s.store.Snapshot()
	next := old
	panelPath, err := normalizePanelPath(in.PanelPath)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	next.Settings.PanelName = cleanText(in.PanelName, "m-ui", 60)
	next.Settings.PanelPath = panelPath
	next.Settings.Domain = strings.TrimSpace(in.Domain)
	if next.Settings.Domain != "" {
		domain, err := normalizeNodeAddress(next.Settings.Domain)
		if err != nil || net.ParseIP(domain) != nil {
			writeError(w, 400, "节点域名格式不正确，请只填写域名，例如 vpn.example.com")
			return
		}
		next.Settings.Domain = domain
	}
	next.Settings.CertFile = cleanPath(in.CertFile, defaultCertFile())
	next.Settings.KeyFile = cleanPath(in.KeyFile, defaultKeyFile())
	next.Settings.AdminUser = cleanText(in.AdminUser, old.Settings.AdminUser, 60)

	accountChanged := next.Settings.AdminUser != old.Settings.AdminUser || in.NewPassword != ""
	if accountChanged {
		if !verifyPassword(in.CurrentPassword, old.Settings.PasswordSalt, old.Settings.PasswordHash) {
			writeError(w, 400, "当前密码不正确")
			return
		}
	}
	if in.NewPassword != "" {
		if in.NewPassword != in.ConfirmPassword {
			writeError(w, 400, "两次输入的新密码不一致")
			return
		}
		if err := setPassword(&next, in.NewPassword); err != nil {
			writeError(w, 400, err.Error())
			return
		}
	}

	portChanged := requestedPort != oldConfiguredPort
	if portChanged {
		if err := s.stagePanelPort(requestedPort); err != nil {
			writeError(w, 400, err.Error())
			return
		}
	}

	if err := s.commitXrayState(old, next); err != nil {
		if portChanged {
			_ = s.stagePanelPort(oldConfiguredPort)
		}
		writeError(w, 400, err.Error())
		return
	}

	pathChanged := next.Settings.PanelPath != old.Settings.PanelPath
	restartRequired := portChanged
	reloginRequired := accountChanged || restartRequired

	expireAuthCookies := func(cookiePath string) {
		p, err := normalizePanelPath(cookiePath)
		if err != nil || p == "" {
			p = "/"
		}
		http.SetCookie(w, &http.Cookie{Name: "mui_session", Value: "", Path: p, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: -1})
		http.SetCookie(w, &http.Cookie{Name: "mui_csrf", Value: "", Path: p, HttpOnly: false, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: -1})
	}

	if reloginRequired {
		expireAuthCookies(old.Settings.PanelPath)
		if next.Settings.PanelPath != old.Settings.PanelPath {
			expireAuthCookies(next.Settings.PanelPath)
		}
		s.sessions.Range(func(k, v any) bool { s.sessions.Delete(k); return true })
	} else if pathChanged {
		// The panel path can change without restarting. Move the existing session
		// cookie to the new path so the user stays signed in after navigation.
		if ck, err := r.Cookie("mui_session"); err == nil && ck.Value != "" {
			if v, ok := s.sessions.Load(ck.Value); ok {
				exp := v.(session).Expires
				p, err := normalizePanelPath(next.Settings.PanelPath)
				if err != nil || p == "" {
					p = "/"
				}
				http.SetCookie(w, &http.Cookie{
					Name: "mui_session", Value: ck.Value, Path: p, HttpOnly: true,
					SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil,
					Expires: exp, MaxAge: int(time.Until(exp).Seconds()),
				})
				if sess, ok := v.(session); ok && sess.CSRF != "" {
					http.SetCookie(w, &http.Cookie{Name: "mui_csrf", Value: sess.CSRF, Path: p, HttpOnly: false, SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, Expires: exp, MaxAge: int(time.Until(exp).Seconds())})
				}
				expireAuthCookies(old.Settings.PanelPath)
			}
		}
	}

	out := makeBootstrap(next)
	out.Settings.PanelPort = requestedPort
	out.RestartRequired = restartRequired
	out.ReloginRequired = reloginRequired
	out.PanelPathChanged = pathChanged
	writeJSON(w, 200, out)

	if restartRequired {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		go func() {
			time.Sleep(1500 * time.Millisecond)
			os.Exit(42)
		}()
	}
}

func (s *Server) handleCreateDefaultNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	old := s.store.Snapshot()
	next := old
	used := map[int]bool{}
	for _, n := range next.Nodes {
		used[n.Port] = true
	}
	port, err := availablePort(used)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	uuid, err := generateUUID()
	if err != nil {
		writeError(w, 500, "UUID 生成失败")
		return
	}
	identity := detectServerIdentity()
	if net.ParseIP(identity.IP) == nil {
		writeError(w, 502, "无法自动识别服务器公网 IP，请检查服务器是否可以访问公网后重试")
		return
	}
	privateKey, password, shortID, err := s.xray.GenerateRealityMaterial()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	address := identity.IP
	if strings.TrimSpace(next.Settings.Domain) != "" {
		domain, err := normalizeNodeAddress(next.Settings.Domain)
		if err != nil || net.ParseIP(domain) != nil {
			writeError(w, 400, "系统设置中的节点域名格式不正确")
			return
		}
		address = domain
	}

	name := nextCountryNodeName(identity.Country, next.Nodes)
	node := Node{
		ID: newID(), Name: name, UUID: uuid, Port: port, Enabled: true,
		Security: "reality", Address: address, Country: normalizeCountryCode(identity.Country),
		RealityPrivateKey: privateKey, RealityPassword: password, RealityShortID: shortID,
		RealityServerName: next.Settings.RealityServerName, RealityTarget: next.Settings.RealityTarget,
		CreatedAt: time.Now(),
	}
	next.Nodes = append(next.Nodes, node)
	if err := s.commitXrayState(old, next); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	warning := tryOpenFirewall(port)
	writeJSON(w, 201, map[string]any{
		"node": NodeView{Node: node, URI: nodeURI(next, node)},
		"warning": warning,
		"country": identity.Country,
		"address": node.Address,
	})
}

func (s *Server) handleNode(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/nodes/"), "/")
	if strings.HasSuffix(id, "/reset-traffic") {
		nodeID := strings.TrimSuffix(id, "/reset-traffic")
		s.handleResetNodeTraffic(w, r, strings.Trim(nodeID, "/"))
		return
	}
	if id == "" {
		writeError(w, 404, "节点不存在")
		return
	}
	old := s.store.Snapshot()
	next := old
	idx := -1
	toggleOnly := false
	for i := range next.Nodes {
		if next.Nodes[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		writeError(w, 404, "节点不存在")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		next.Nodes = append(next.Nodes[:idx], next.Nodes[idx+1:]...)
	case http.MethodPatch:
		var in struct {
			Name              *string `json:"name"`
			Enabled           *bool   `json:"enabled"`
			Address           *string `json:"address"`
			Port              *int    `json:"port"`
			RealityServerName *string `json:"reality_server_name"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		toggleOnly = in.Enabled != nil && in.Name == nil && in.Address == nil && in.Port == nil && in.RealityServerName == nil
		if in.Name != nil {
			next.Nodes[idx].Name = cleanText(*in.Name, next.Nodes[idx].Name, 60)
		}
		if in.Enabled != nil {
			next.Nodes[idx].Enabled = *in.Enabled
		}
		if in.Address != nil {
			address, err := normalizeNodeAddress(*in.Address)
			if err != nil {
				writeError(w, 400, err.Error())
				return
			}
			next.Nodes[idx].Address = address
		}
		if in.Port != nil {
			if *in.Port < 1024 || *in.Port > 65535 {
				writeError(w, 400, "节点端口必须在 1024-65535 之间")
				return
			}
			if *in.Port != next.Nodes[idx].Port {
				if err := validateEditedPort(*in.Port, next.Nodes[idx].ID, next.Nodes); err != nil {
					writeError(w, 400, err.Error())
					return
				}
				next.Nodes[idx].Port = *in.Port
			}
		}
		if in.RealityServerName != nil {
			if nodeSecurity(next.Nodes[idx]) != "reality" {
				writeError(w, 400, "只有 REALITY 节点可以修改伪装域名")
				return
			}
			serverName, err := normalizeDNSName(*in.RealityServerName)
			if err != nil {
				writeError(w, 400, "REALITY 伪装域名格式不正确")
				return
			}
			next.Nodes[idx].RealityServerName = serverName
			next.Nodes[idx].RealityTarget = serverName + ":443"
		}
	default:
		methodNotAllowed(w)
		return
	}
	if toggleOnly {
		if err := s.commitNodeToggleWithEarlyResponse(w, old, next); err != nil {
			writeError(w, 400, err.Error())
		}
		return
	}
	if err := s.commitXrayState(old, next); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if r.Method == http.MethodPatch && next.Nodes[idx].Port != old.Nodes[idx].Port {
		_ = tryOpenFirewall(next.Nodes[idx].Port)
	}
	writeJSON(w, 200, makeBootstrap(next))
}

func normalizeSubscriptionSlug(v string) (string, error) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "/sub/")
	v = strings.Trim(v, "/")
	if len(v) < 20 || len(v) > 80 {
		return "", errors.New("订阅地址后缀长度应为 20-80 位")
	}
	for _, r := range v {
		ok := r == '-' || r == '_' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return "", errors.New("订阅地址只能包含字母、数字、- 和 _")
		}
	}
	return v, nil
}

func subscriptionSlugExists(st AppState, slug, exceptID string) bool {
	for _, sub := range st.Subscriptions {
		if sub.ID != exceptID && strings.EqualFold(sub.Slug, slug) {
			return true
		}
	}
	return false
}

func normalizeSubscriptionNodeIDs(st AppState, ids []string) ([]string, error) {
	known := make(map[string]bool, len(st.Nodes))
	for _, node := range st.Nodes {
		known[node.ID] = true
	}
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if !known[id] {
			return nil, fmt.Errorf("节点不存在: %s", id)
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

func subscriptionHasNode(sub Subscription, nodeID string) bool {
	if sub.IsDefault {
		return true
	}
	for _, id := range sub.NodeIDs {
		if id == nodeID {
			return true
		}
	}
	return false
}

func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, 200, s.store.Snapshot().Subscriptions)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var in struct {
		Name    string   `json:"name"`
		Slug    string   `json:"slug"`
		NodeIDs []string `json:"node_ids"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	var sub Subscription
	err := s.store.Update(func(st *AppState) error {
		token := randomToken(28)
		slug := strings.TrimSpace(in.Slug)
		if slug == "" {
			slug = token
		} else {
			var err error
			slug, err = normalizeSubscriptionSlug(slug)
			if err != nil {
				return err
			}
		}
		if subscriptionSlugExists(*st, slug, "") {
			return errors.New("该订阅地址已被使用")
		}
		nodeIDs, err := normalizeSubscriptionNodeIDs(*st, in.NodeIDs)
		if err != nil {
			return err
		}
		sub = Subscription{
			ID: newID(), Name: cleanText(in.Name, "新订阅", 60), Token: token, Slug: slug,
			Enabled: true, IsDefault: false, NodeIDs: nodeIDs, CreatedAt: time.Now(),
		}
		st.Subscriptions = append(st.Subscriptions, sub)
		return nil
	})
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, sub)
}

func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/subscriptions/"), "/")
	if id == "" {
		writeError(w, 404, "订阅不存在")
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var in struct {
			Name    *string   `json:"name"`
			Slug    *string   `json:"slug"`
			NodeIDs *[]string `json:"node_ids"`
			Enabled *bool     `json:"enabled"`
		}
		if !decodeJSON(w, r, &in) {
			return
		}
		err := s.store.Update(func(st *AppState) error {
			idx := -1
			for i := range st.Subscriptions {
				if st.Subscriptions[i].ID == id {
					idx = i
					break
				}
			}
			if idx < 0 {
				return errors.New("订阅不存在")
			}
			if in.Name != nil {
				st.Subscriptions[idx].Name = cleanText(*in.Name, st.Subscriptions[idx].Name, 60)
			}
			if in.Slug != nil {
				slug, err := normalizeSubscriptionSlug(*in.Slug)
				if err != nil {
					return err
				}
				if subscriptionSlugExists(*st, slug, id) {
					return errors.New("该订阅地址已被使用")
				}
				st.Subscriptions[idx].Slug = slug
			}
			if in.Enabled != nil {
				st.Subscriptions[idx].Enabled = *in.Enabled
			}
			if !st.Subscriptions[idx].IsDefault && in.NodeIDs != nil {
				nodeIDs, err := normalizeSubscriptionNodeIDs(*st, *in.NodeIDs)
				if err != nil {
					return err
				}
				st.Subscriptions[idx].NodeIDs = nodeIDs
			}
			return nil
		})
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, makeBootstrap(s.store.Snapshot()))
	case http.MethodDelete:
		err := s.store.Update(func(st *AppState) error {
			idx := -1
			for i := range st.Subscriptions {
				if st.Subscriptions[i].ID == id {
					idx = i
					break
				}
			}
			if idx < 0 {
				return errors.New("订阅不存在")
			}
			if st.Subscriptions[idx].IsDefault {
				return errors.New("默认订阅不能删除")
			}
			st.Subscriptions = append(st.Subscriptions[:idx], st.Subscriptions[idx+1:]...)
			return nil
		})
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleSubscriptionFeed(w http.ResponseWriter, r *http.Request) {
	token := strings.Trim(strings.TrimPrefix(r.URL.Path, "/sub/"), "/")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	st := s.store.Snapshot()
	idx := -1
	for i := range st.Subscriptions {
		if st.Subscriptions[i].Slug == token && st.Subscriptions[i].Enabled {
			idx = i
			break
		}
	}
	if idx < 0 {
		http.NotFound(w, r)
		return
	}
	sub := st.Subscriptions[idx]
	var lines []string
	for _, n := range st.Nodes {
		if n.Enabled && subscriptionHasNode(sub, n.ID) {
			lines = append(lines, nodeURI(st, n))
		}
	}
	raw := strings.Join(lines, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(encoded))
}

func trafficDeltasByNodeID(st AppState, raw map[string]TrafficDelta) map[string]TrafficDelta {
	out := map[string]TrafficDelta{}
	for _, n := range st.Nodes {
		if d, ok := raw[n.ID+"@m-ui"]; ok {
			out[n.ID] = d
		}
	}
	return out
}

func applyNodeTrafficDeltas(st *AppState, deltas map[string]TrafficDelta) {
	for i := range st.Nodes {
		if d, ok := deltas[st.Nodes[i].ID]; ok {
			st.Nodes[i].UplinkBytes += d.Uplink
			st.Nodes[i].DownlinkBytes += d.Downlink
		}
	}
}

func (s *Server) syncTraffic() error {
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()
	raw, err := s.xray.QueryUserTraffic(true)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	return s.store.Update(func(st *AppState) error {
		applyNodeTrafficDeltas(st, trafficDeltasByNodeID(*st, raw))
		return nil
	})
}

func (s *Server) trafficLoop() {
	timer := time.NewTicker(60 * time.Second)
	defer timer.Stop()
	for range timer.C {
		if err := s.syncTraffic(); err != nil {
			log.Printf("m-ui: 节点流量采集暂不可用: %v", err)
		}
	}
}

func (s *Server) handleResetAllTraffic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()
	_, _ = s.xray.QueryUserTraffic(true)
	err := s.store.Update(func(st *AppState) error {
		for i := range st.Nodes {
			st.Nodes[i].UplinkBytes = 0
			st.Nodes[i].DownlinkBytes = 0
		}
		return nil
	})
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, makeBootstrap(s.store.Snapshot()))
}

func (s *Server) handleResetNodeTraffic(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if id == "" {
		writeError(w, 404, "节点不存在")
		return
	}
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()
	st := s.store.Snapshot()
	raw, _ := s.xray.QueryUserTraffic(true)
	applyNodeTrafficDeltas(&st, trafficDeltasByNodeID(st, raw))
	found := false
	for i := range st.Nodes {
		if st.Nodes[i].ID == id {
			st.Nodes[i].UplinkBytes = 0
			st.Nodes[i].DownlinkBytes = 0
			found = true
			break
		}
	}
	if !found {
		writeError(w, 404, "节点不存在")
		return
	}
	if err := s.store.Replace(st); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, makeBootstrap(st))
}

func (s *Server) commitNodeToggleWithEarlyResponse(w http.ResponseWriter, old, next AppState) error {
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()

	if raw, err := s.xray.QueryUserTraffic(true); err == nil {
		deltas := trafficDeltasByNodeID(old, raw)
		applyNodeTrafficDeltas(&old, deltas)
		applyNodeTrafficDeltas(&next, deltas)
	}
	if err := s.xray.Validate(next); err != nil {
		return err
	}
	if err := s.store.Replace(next); err != nil {
		return err
	}

	writeJSONAndFlush(w, http.StatusOK, makeBootstrap(next))
	// Give the browser time to receive the complete response before Xray is restarted.
	// This matters when the administrator is currently reaching the panel through
	// the very node that is being stopped.
	time.Sleep(250 * time.Millisecond)

	if err := s.xray.HotApply(old, next); err != nil {
		log.Printf("m-ui: 节点状态切换失败，正在回滚: %v", err)
		_ = s.store.Replace(old)
		_ = s.xray.Apply(old)
	}
	return nil
}

func (s *Server) commitXrayState(old, next AppState) error {
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()

	if raw, err := s.xray.QueryUserTraffic(true); err == nil {
		deltas := trafficDeltasByNodeID(old, raw)
		applyNodeTrafficDeltas(&old, deltas)
		applyNodeTrafficDeltas(&next, deltas)
	}
	if err := s.store.Replace(next); err != nil {
		return err
	}
	if err := s.xray.HotApply(old, next); err != nil {
		_ = s.store.Replace(old)
		_ = s.xray.Apply(old)
		return err
	}
	return nil
}

func (s *Server) loginBlocked(ip string) time.Duration {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	f := s.failures[ip]
	if f == nil {
		return 0
	}
	if f.Until.After(time.Now()) {
		return time.Until(f.Until)
	}
	if !f.Until.IsZero() {
		delete(s.failures, ip)
	}
	return 0
}
func (s *Server) recordLoginFailure(ip string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	f := s.failures[ip]
	if f == nil {
		if len(s.failures) >= 4096 {
			now := time.Now()
			for key, item := range s.failures {
				if item.Until.IsZero() || item.Until.Before(now) {
					delete(s.failures, key)
				}
				if len(s.failures) < 3072 {
					break
				}
			}
		}
		f = &loginFailure{}
		s.failures[ip] = f
	}
	f.Count++
	if f.Count >= 5 {
		f.Until = time.Now().Add(5 * time.Minute)
		f.Count = 0
	}
}
func (s *Server) clearLoginFailure(ip string) {
	s.loginMu.Lock()
	delete(s.failures, ip)
	s.loginMu.Unlock()
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(strings.TrimSpace(host))
	if peer != nil && (peer.IsLoopback() || peer.IsPrivate()) {
		if x := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); x != "" {
			if forwarded := net.ParseIP(x); forwarded != nil {
				return forwarded.String()
			}
		}
		if x := strings.TrimSpace(r.Header.Get("X-Real-IP")); x != "" {
			if forwarded := net.ParseIP(x); forwarded != nil {
				return forwarded.String()
			}
		}
	}
	if peer != nil {
		return peer.String()
	}
	return host
}

func cleanText(v, fallback string, max int) string {
	v = strings.TrimSpace(v)
	if v == "" {
		v = fallback
	}
	r := []rune(v)
	if len(r) > max {
		v = string(r[:max])
	}
	return v
}
func cleanPath(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return filepath.Clean(v)
}

func normalizeNodeAddress(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", errors.New("节点地址不能为空")
	}
	ipText := strings.Trim(v, "[]")
	if ip := net.ParseIP(ipText); ip != nil {
		return ip.String(), nil
	}
	return normalizeDNSName(v)
}

func normalizeDNSName(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(v, ".")))
	if v == "" || len(v) > 253 || strings.ContainsAny(v, "/?#@: \t\r\n") {
		return "", errors.New("域名格式不正确")
	}
	labels := strings.Split(v, ".")
	if len(labels) < 2 {
		return "", errors.New("域名至少需要包含一个点")
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("域名格式不正确")
		}
		for _, r := range label {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return "", errors.New("域名只能包含字母、数字、点和连字符")
			}
		}
	}
	return v, nil
}

func validateEditedPort(port int, currentID string, nodes []Node) error {
	if strings.Contains(strconv.Itoa(port), "4") {
		return errors.New("节点端口不能包含数字 4")
	}
	for _, n := range nodes {
		if n.ID != currentID && n.Port == port {
			return fmt.Errorf("端口 %d 已被其他节点使用", port)
		}
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("端口 %d 已被系统中的其他程序占用", port)
	}
	_ = ln.Close()
	return nil
}

func randomSessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return randomToken(48)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, 400, "请求格式不正确")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONAndFlush(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "响应编码失败")
		return
	}
	b = append(b, '\n')
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(b)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func init() { log.SetFlags(log.Ldate | log.Ltime | log.LUTC) }
