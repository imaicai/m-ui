package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const stateVersion = 4

func defaultCertHome() string {
	if v := strings.TrimSpace(os.Getenv("MUI_CERT_HOME")); v != "" {
		return filepath.Clean(v)
	}
	if v, err := os.UserHomeDir(); err == nil && strings.TrimSpace(v) != "" {
		return filepath.Clean(v)
	}
	return "/root"
}

func defaultCertFile() string {
	return filepath.Join(defaultCertHome(), "minijet", "ssh", "m-ui", "fullchain.pem")
}

func defaultKeyFile() string {
	return filepath.Join(defaultCertHome(), "minijet", "ssh", "m-ui", "privkey.pem")
}


type Settings struct {
	PanelName         string `json:"panel_name"`
	PanelPath         string `json:"panel_path"`
	Domain            string `json:"domain"`
	CertFile          string `json:"cert_file"`
	KeyFile           string `json:"key_file"`
	DefaultProtocol   string `json:"default_protocol"`
	RealityTarget     string `json:"reality_target"`
	RealityServerName string `json:"reality_server_name"`
	AdminUser         string `json:"admin_user"`
	PasswordSalt      string `json:"password_salt"`
	PasswordHash      string `json:"password_hash"`
}

type Node struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	UUID              string    `json:"uuid"`
	Port              int       `json:"port"`
	Enabled           bool      `json:"enabled"`
	Security          string    `json:"security,omitempty"`
	Address           string    `json:"address,omitempty"`
	RealityPrivateKey string    `json:"reality_private_key,omitempty"`
	RealityPassword   string    `json:"reality_password,omitempty"`
	RealityShortID    string    `json:"reality_short_id,omitempty"`
	RealityServerName string    `json:"reality_server_name,omitempty"`
	RealityTarget     string    `json:"reality_target,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UplinkBytes       uint64    `json:"uplink_bytes,omitempty"`
	DownlinkBytes     uint64    `json:"downlink_bytes,omitempty"`
}

type Subscription struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	Slug      string    `json:"slug"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type AppState struct {
	Version       int            `json:"version"`
	Settings      Settings       `json:"settings"`
	Nodes         []Node         `json:"nodes"`
	Subscriptions []Subscription `json:"subscriptions"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data AppState
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.data = defaultState()
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return fmt.Errorf("读取数据文件失败: %w", err)
	}

	// Upgrade older state files without breaking existing nodes.
	if s.data.Settings.PanelPath == "" {
		s.data.Settings.PanelPath = "/"
	}
	if s.data.Settings.CertFile == "" ||
		s.data.Settings.CertFile == "/maicai/ssh/fullchain.pem" ||
		s.data.Settings.CertFile == "/maicai/ssh/m-ui/fullchain.pem" ||
		s.data.Settings.CertFile == "/root/minijet/ssh/m-ui/fullchain.pem" {
		s.data.Settings.CertFile = defaultCertFile()
	}
	if s.data.Settings.KeyFile == "" ||
		s.data.Settings.KeyFile == "/maicai/ssh/privkey.pem" ||
		s.data.Settings.KeyFile == "/maicai/ssh/m-ui/privkey.pem" ||
		s.data.Settings.KeyFile == "/root/minijet/ssh/m-ui/privkey.pem" {
		s.data.Settings.KeyFile = defaultKeyFile()
	}
	if s.data.Settings.DefaultProtocol == "" || strings.Contains(s.data.Settings.DefaultProtocol, "TLS") {
		s.data.Settings.DefaultProtocol = "VLESS + RAW + REALITY + XTLS Vision"
	}
	if s.data.Settings.RealityTarget == "" || s.data.Settings.RealityTarget == "www.microsoft.com:443" {
		s.data.Settings.RealityTarget = "www.apple.com:443"
	}
	if s.data.Settings.RealityServerName == "" || s.data.Settings.RealityServerName == "www.microsoft.com" {
		s.data.Settings.RealityServerName = "www.apple.com"
	}
	for i := range s.data.Nodes {
		if s.data.Nodes[i].Security == "" {
			s.data.Nodes[i].Security = "tls"
		}
	}
	for i := range s.data.Subscriptions {
		if strings.TrimSpace(s.data.Subscriptions[i].Slug) == "" {
			s.data.Subscriptions[i].Slug = s.data.Subscriptions[i].Token
		}
	}
	s.data.Version = stateVersion
	return nil
}

func defaultState() AppState {
	return AppState{
		Version: stateVersion,
		Settings: Settings{
			PanelName:         "m-ui",
			PanelPath:         "",
			CertFile:          defaultCertFile(),
			KeyFile:           defaultKeyFile(),
			DefaultProtocol:   "VLESS + RAW + REALITY + XTLS Vision",
			RealityTarget:     "www.apple.com:443",
			RealityServerName: "www.apple.com",
			AdminUser:         "admin",
		},
		Nodes:         []Node{},
		Subscriptions: []Subscription{},
	}
}

func (s *Store) Snapshot() AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.data)
	var cp AppState
	_ = json.Unmarshal(b, &cp)
	return cp
}

func (s *Store) Update(fn func(*AppState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, _ := json.Marshal(s.data)
	if err := fn(&s.data); err != nil {
		return err
	}
	if err := s.saveLocked(); err != nil {
		_ = json.Unmarshal(before, &s.data)
		return err
	}
	return nil
}

func (s *Store) Replace(data AppState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) EnsureInitialized(user, pass string) error {
	return s.Update(func(st *AppState) error {
		if st.Settings.PasswordHash != "" {
			return nil
		}
		user = strings.TrimSpace(user)
		if user == "" {
			user = "admin"
		}
		if pass == "" {
			return errors.New("初始化密码不能为空")
		}
		salt, hash, err := hashPassword(pass)
		if err != nil {
			return err
		}
		st.Settings.AdminUser = user
		if st.Settings.PanelPath == "" {
			st.Settings.PanelPath = randomPanelPath()
		}
		st.Settings.PasswordSalt = salt
		st.Settings.PasswordHash = hash
		if len(st.Subscriptions) == 0 {
			token := randomToken(24)
			st.Subscriptions = append(st.Subscriptions, Subscription{
				ID: newID(), Name: "默认订阅", Token: token, Slug: token, Enabled: true, CreatedAt: time.Now(),
			})
		}
		return nil
	})
}

func hashPassword(password string) (saltHex, hashHex string, err error) {
	salt := make([]byte, 16)
	if _, err = rand.Read(salt); err != nil {
		return "", "", err
	}
	h := passwordKDF(password, salt)
	return hex.EncodeToString(salt), hex.EncodeToString(h), nil
}

func passwordKDF(password string, salt []byte) []byte {
	sum := sha256.Sum256(append(append([]byte{}, salt...), []byte(password)...))
	v := sum[:]
	for i := 0; i < 120000; i++ {
		x := sha256.Sum256(append(append([]byte{}, v...), salt...))
		v = x[:]
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out
}

func verifyPassword(password, saltHex, hashHex string) bool {
	salt, err1 := hex.DecodeString(saltHex)
	expected, err2 := hex.DecodeString(hashHex)
	if err1 != nil || err2 != nil || len(expected) == 0 {
		return false
	}
	got := passwordKDF(password, salt)
	if len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare(got, expected) == 1
}

func setPassword(st *AppState, password string) error {
	if len(password) < 8 {
		return errors.New("新密码至少 8 位")
	}
	salt, hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	st.Settings.PasswordSalt = salt
	st.Settings.PasswordHash = hash
	return nil
}

func randomToken(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	rnd := make([]byte, n)
	if _, err := rand.Read(rnd); err != nil {
		now := time.Now().UnixNano()
		for i := range b {
			b[i] = alphabet[(int(now)+i*31)%len(alphabet)]
		}
		return string(b)
	}
	for i := range b {
		b[i] = alphabet[int(rnd[i])%len(alphabet)]
	}
	return string(b)
}

func newID() string { return strings.ToLower(randomToken(12)) }

func randomPanelPath() string {
	return "/" + strings.ToLower(randomToken(12)) + "/"
}

func normalizePanelPath(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || v == "/" {
		return "/", nil
	}
	if !strings.HasPrefix(v, "/") {
		v = "/" + v
	}
	if !strings.HasSuffix(v, "/") {
		v += "/"
	}
	if len(v) > 80 || strings.Contains(v, "..") {
		return "", errors.New("面板 URL 根路径格式不正确")
	}
	for _, r := range strings.Trim(v, "/") {
		ok := r == '/' || r == '-' || r == '_' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return "", errors.New("面板 URL 根路径只能包含字母、数字、-、_ 和 /")
		}
	}
	return v, nil
}
