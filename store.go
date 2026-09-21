package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"golang.org/x/crypto/argon2"
)

const stateVersion = 6

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
	Country           string    `json:"country,omitempty"`
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
	IsDefault bool      `json:"is_default,omitempty"`
	NodeIDs   []string  `json:"node_ids"`
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

func allNodeIDs(nodes []Node) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.ID) != "" {
			ids = append(ids, node.ID)
		}
	}
	return ids
}

func normalizeSubscriptions(st *AppState, migrateLegacySelection bool) {
	if st == nil || len(st.Subscriptions) == 0 {
		return
	}

	defaultIdx := -1
	for i := range st.Subscriptions {
		if st.Subscriptions[i].IsDefault {
			if defaultIdx < 0 {
				defaultIdx = i
			} else {
				st.Subscriptions[i].IsDefault = false
			}
		}
	}
	if defaultIdx < 0 {
		defaultIdx = 0
		st.Subscriptions[0].IsDefault = true
	}
	if defaultIdx != 0 {
		def := st.Subscriptions[defaultIdx]
		copy(st.Subscriptions[1:defaultIdx+1], st.Subscriptions[0:defaultIdx])
		st.Subscriptions[0] = def
	}

	allIDs := allNodeIDs(st.Nodes)
	valid := make(map[string]bool, len(allIDs))
	for _, id := range allIDs {
		valid[id] = true
	}
	for i := range st.Subscriptions {
		if i == 0 {
			st.Subscriptions[i].IsDefault = true
			st.Subscriptions[i].NodeIDs = append([]string{}, allIDs...)
			continue
		}
		st.Subscriptions[i].IsDefault = false
		ids := st.Subscriptions[i].NodeIDs
		if migrateLegacySelection && ids == nil {
			ids = allIDs
		}
		cleaned := make([]string, 0, len(ids))
		seen := map[string]bool{}
		for _, id := range ids {
			if valid[id] && !seen[id] {
				seen[id] = true
				cleaned = append(cleaned, id)
			}
		}
		st.Subscriptions[i].NodeIDs = cleaned
	}
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
	migrateSubscriptionNodeSelection := s.data.Version < 5

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
	normalizeSubscriptions(&s.data, migrateSubscriptionNodeSelection)
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
	normalizeSubscriptions(&s.data, false)
	s.data.Version = stateVersion
	if err := s.saveLocked(); err != nil {
		_ = json.Unmarshal(before, &s.data)
		return err
	}
	return nil
}

func (s *Store) Replace(data AppState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalizeSubscriptions(&data, false)
	data.Version = stateVersion
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
				ID: newID(), Name: "默认订阅", Token: token, Slug: token, Enabled: true, IsDefault: true, NodeIDs: []string{}, CreatedAt: time.Now(),
			})
		}
		return nil
	})
}

const (
	argonMemory  uint32 = 32 * 1024
	argonTime    uint32 = 2
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
)

func hashPassword(password string) (saltHex, hashEncoded string, err error) {
	salt := make([]byte, 16)
	if _, err = rand.Read(salt); err != nil {
		return "", "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	// Keep PasswordSalt populated for backwards-compatible state shape. New
	// verification uses the self-contained Argon2id hash string.
	return hex.EncodeToString(salt), encoded, nil
}

func passwordNeedsUpgrade(hash string) bool {
	return !strings.HasPrefix(strings.TrimSpace(hash), "$argon2id$")
}

func verifyArgon2Password(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, iterations uint32
	var threads64 uint64
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false
	}
	if _, err := fmt.Sscanf(params[0], "m=%d", &memory); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(params[1], "t=%d", &iterations); err != nil {
		return false
	}
	pText := strings.TrimPrefix(params[2], "p=")
	var err error
	threads64, err = strconv.ParseUint(pText, 10, 8)
	if err != nil {
		return false
	}
	threads := uint8(threads64)
	if memory < 8*1024 || memory > 256*1024 || iterations < 1 || iterations > 10 || threads < 1 || threads > 4 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(got, expected) == 1
}

func verifyPassword(password, saltHex, hashValue string) bool {
	if strings.HasPrefix(strings.TrimSpace(hashValue), "$argon2id$") {
		return verifyArgon2Password(password, hashValue)
	}
	// Legacy v1.x SHA-256 KDF. Successful logins are transparently upgraded.
	salt, err1 := hex.DecodeString(saltHex)
	expected, err2 := hex.DecodeString(hashValue)
	if err1 != nil || err2 != nil || len(expected) == 0 {
		return false
	}
	got := legacyPasswordKDF(password, salt)
	if len(got) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare(got, expected) == 1
}

func legacyPasswordKDF(password string, salt []byte) []byte {
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
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, n)
	buf := make([]byte, 64)
	const maxUnbiased = byte(248) // 62 * 4; reject 248-255 to remove modulo bias.
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			panic("m-ui: secure random source unavailable")
		}
		for _, v := range buf {
			if v >= maxUnbiased {
				continue
			}
			out = append(out, alphabet[int(v)%len(alphabet)])
			if len(out) == n {
				break
			}
		}
	}
	return string(out)
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
