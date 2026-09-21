package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const xrayStatsAPIAddr = "127.0.0.1:10085"

type TrafficDelta struct {
	Uplink   uint64
	Downlink uint64
}

type XrayManager struct {
	bin        string
	configPath string
}

func NewXrayManager(bin, configPath string) *XrayManager {
	return &XrayManager{bin: bin, configPath: configPath}
}

func (x *XrayManager) BuildConfig(st AppState) ([]byte, error) {
	enabled := make([]Node, 0, len(st.Nodes))
	for _, n := range st.Nodes {
		if n.Enabled {
			enabled = append(enabled, n)
		}
	}

	inbounds := make([]any, 0, len(enabled))
	for _, n := range enabled {
		security := nodeSecurity(n)
		stream := map[string]any{
			"method": "raw",
		}
		switch security {
		case "reality":
			if strings.TrimSpace(n.RealityPrivateKey) == "" || strings.TrimSpace(n.RealityPassword) == "" || strings.TrimSpace(n.RealityShortID) == "" {
				return nil, fmt.Errorf("节点 %s 的 REALITY 密钥不完整", n.Name)
			}
			target := strings.TrimSpace(n.RealityTarget)
			if target == "" {
				target = st.Settings.RealityTarget
			}
			serverName := strings.TrimSpace(n.RealityServerName)
			if serverName == "" {
				serverName = st.Settings.RealityServerName
			}
			if target == "" || serverName == "" {
				return nil, fmt.Errorf("节点 %s 的 REALITY 伪装目标未设置", n.Name)
			}
			stream["security"] = "reality"
			stream["realitySettings"] = map[string]any{
				"show":       false,
				"target":     target,
				"xver":       0,
				"serverNames": []string{serverName},
				"privateKey":    n.RealityPrivateKey,
				"shortIds":      []string{n.RealityShortID},
				"minClientVer": "1.8.2",
				"limitFallbackUpload": map[string]any{
					"afterBytes": 10485760, "bytesPerSec": 1048576, "burstBytesPerSec": 5242880,
				},
				"limitFallbackDownload": map[string]any{
					"afterBytes": 10485760, "bytesPerSec": 1048576, "burstBytesPerSec": 5242880,
				},
			}
		default:
			if strings.TrimSpace(st.Settings.Domain) == "" {
				return nil, fmt.Errorf("节点 %s 使用 TLS，请先在系统设置中填写节点域名", n.Name)
			}
			if err := readableFile(st.Settings.CertFile); err != nil {
				return nil, fmt.Errorf("证书文件不可用: %w", err)
			}
			if err := readableFile(st.Settings.KeyFile); err != nil {
				return nil, fmt.Errorf("私钥文件不可用: %w", err)
			}
			stream["security"] = "tls"
			stream["tlsSettings"] = map[string]any{
				"alpn": []string{"h2", "http/1.1"},
				"certificates": []any{map[string]any{
					"certificateFile": st.Settings.CertFile,
					"keyFile":         st.Settings.KeyFile,
				}},
			}
		}

		inbounds = append(inbounds, map[string]any{
			"listen":   "0.0.0.0",
			"port":     n.Port,
			"protocol": "vless",
			"tag":      "m-ui-" + n.ID,
			"settings": map[string]any{
				"clients": []any{map[string]any{
					"id":    n.UUID,
					"flow":  "xtls-rprx-vision",
					"email": n.ID + "@m-ui",
				}},
				"decryption": "none",
			},
			"streamSettings": stream,
		})
	}

	inbounds = append(inbounds, map[string]any{
		"listen":   "127.0.0.1",
		"port":     10085,
		"protocol": "dokodemo-door",
		"tag":      "api-in",
		"settings": map[string]any{"address": "127.0.0.1"},
	})

	cfg := map[string]any{
		"log":      map[string]any{"loglevel": "warning"},
		"stats":    map[string]any{},
		"policy": map[string]any{
			"levels": map[string]any{
				"0": map[string]any{
					"statsUserUplink":   true,
					"statsUserDownlink": true,
				},
			},
		},
		"api": map[string]any{
			"tag":      "api",
			"services": []string{"StatsService"},
		},
		"inbounds": inbounds,
		"outbounds": []any{
			map[string]any{"protocol": "freedom", "tag": "direct"},
			map[string]any{"protocol": "blackhole", "tag": "blocked"},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []any{
				map[string]any{"type": "field", "inboundTag": []string{"api-in"}, "outboundTag": "api"},
				map[string]any{"type": "field", "protocol": []string{"bittorrent"}, "outboundTag": "blocked"},
			},
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func (x *XrayManager) QueryUserTraffic(reset bool) (map[string]TrafficDelta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	args := []string{"api", "statsquery", "--server=" + xrayStatsAPIAddr, "-pattern", "user>>>"}
	if reset {
		args = append(args, "-reset")
	}
	out, err := exec.CommandContext(ctx, x.bin, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("读取 Xray 流量统计失败: %s", strings.TrimSpace(string(out)))
	}
	var resp struct {
		Stat []struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		} `json:"stat"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("解析 Xray 流量统计失败: %w", err)
	}
	result := map[string]TrafficDelta{}
	for _, item := range resp.Stat {
		parts := strings.Split(item.Name, ">>>")
		if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
			continue
		}
		email := parts[1]
		raw := strings.Trim(strings.TrimSpace(string(item.Value)), "\"")
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			continue
		}
		d := result[email]
		switch parts[3] {
		case "uplink":
			d.Uplink += value
		case "downlink":
			d.Downlink += value
		}
		result[email] = d
	}
	return result, nil
}

func nodeSecurity(n Node) string {
	if strings.EqualFold(strings.TrimSpace(n.Security), "reality") {
		return "reality"
	}
	return "tls"
}

func (x *XrayManager) validatedTempConfig(st AppState) (string, error) {
	b, err := x.BuildConfig(st)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(x.bin); err != nil {
		return "", fmt.Errorf("未找到 Xray: %s", x.bin)
	}
	if err := os.MkdirAll(filepath.Dir(x.configPath), 0755); err != nil {
		return "", err
	}
	tmp := x.validationTempPath()
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	test := exec.CommandContext(ctx, x.bin, "run", "-test", "-config", tmp)
	if out, err := test.CombinedOutput(); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("Xray 配置校验失败: %s", strings.TrimSpace(string(out)))
	}
	return tmp, nil
}

func (x *XrayManager) Validate(st AppState) error {
	tmp, err := x.validatedTempConfig(st)
	if tmp != "" {
		_ = os.Remove(tmp)
	}
	return err
}

func (x *XrayManager) Apply(st AppState) error {
	tmp, err := x.validatedTempConfig(st)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	if err := os.Rename(tmp, x.configPath); err != nil {
		return err
	}
	if err := os.Chmod(x.configPath, 0644); err != nil {
		return err
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	cmd := exec.CommandContext(ctx2, "systemctl", "restart", "xray")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("重启 Xray 失败: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (x *XrayManager) validationTempPath() string {
	return x.configPath + ".m-ui.tmp.json"
}

func readableFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("路径为空")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}

func generateUUID() (string, error) {
	for attempt := 0; attempt < 256; attempt++ {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		// Use a valid UUIDv5-shaped random ID so the UUID itself does not contain the
		// mandatory version-4 digit. Xray accepts any valid UUID for VLESS IDs.
		b[6] = (b[6] & 0x0f) | 0x50
		b[8] = (b[8] & 0x3f) | 0x80
		h := hex.EncodeToString(b)
		id := fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
		if !strings.Contains(id, "4") {
			return id, nil
		}
	}
	return "", errors.New("无法生成不含数字 4 的 UUID")
}

func availablePort(used map[int]bool) (int, error) {
	for i := 0; i < 1200; i++ {
		rb := make([]byte, 2)
		if _, err := rand.Read(rb); err != nil {
			return 0, err
		}
		p := 10000 + int(rb[0])<<8 + int(rb[1])
		p = 10000 + (p % 50000)
		if used[p] || strings.Contains(strconv.Itoa(p), "4") {
			continue
		}
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(p))
		if err == nil {
			_ = ln.Close()
			return p, nil
		}
	}
	return 0, errors.New("无法自动找到不含数字 4 的可用端口")
}

func randomHexWithoutFour(bytesLen int) (string, error) {
	for attempt := 0; attempt < 256; attempt++ {
		b := make([]byte, bytesLen)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		v := hex.EncodeToString(b)
		if !strings.Contains(v, "4") {
			return v, nil
		}
	}
	return "", errors.New("无法生成不含数字 4 的随机参数")
}

func (x *XrayManager) GenerateRealityMaterial() (privateKey, password, shortID string, err error) {
	for attempt := 0; attempt < 128; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		out, runErr := exec.CommandContext(ctx, x.bin, "x25519").CombinedOutput()
		cancel()
		if runErr != nil {
			return "", "", "", fmt.Errorf("REALITY 密钥生成失败: %s", strings.TrimSpace(string(out)))
		}

		privateKey, password = "", ""
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			lower := strings.ToLower(line)
			switch {
			case strings.HasPrefix(lower, "privatekey:") || strings.HasPrefix(lower, "private key:"):
				privateKey = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			case strings.HasPrefix(lower, "password (publickey):") || strings.HasPrefix(lower, "password:") || strings.HasPrefix(lower, "public key:"):
				password = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			}
		}
		if privateKey == "" || password == "" {
			return "", "", "", fmt.Errorf("无法解析 Xray x25519 输出: %s", strings.TrimSpace(string(out)))
		}
		if strings.Contains(privateKey, "4") || strings.Contains(password, "4") {
			continue
		}
		shortID, err = randomHexWithoutFour(8)
		if err != nil {
			return "", "", "", err
		}
		return privateKey, password, shortID, nil
	}
	return "", "", "", errors.New("无法生成不含数字 4 的 REALITY 密钥参数")
}

func nodeURI(st AppState, n Node) string {
	q := url.Values{}
	q.Set("type", "tcp")
	q.Set("encryption", "none")
	q.Set("flow", "xtls-rprx-vision")

	host := strings.TrimSpace(n.Address)
	if nodeSecurity(n) == "reality" {
		if host == "" {
			host = strings.TrimSpace(st.Settings.Domain)
		}
		q.Set("security", "reality")
		q.Set("sni", n.RealityServerName)
		q.Set("fp", "chrome")
		q.Set("pbk", n.RealityPassword)
		q.Set("sid", n.RealityShortID)
		q.Set("spx", "/")
	} else {
		if host == "" {
			host = strings.TrimSpace(st.Settings.Domain)
		}
		q.Set("security", "tls")
		q.Set("sni", st.Settings.Domain)
		q.Set("alpn", "h2,http/1.1")
	}
	endpoint := net.JoinHostPort(host, strconv.Itoa(n.Port))
	return fmt.Sprintf("vless://%s@%s?%s#%s", n.UUID, endpoint, q.Encode(), url.PathEscape(n.Name))
}

func tryOpenFirewall(port int) string {
	p := strconv.Itoa(port) + "/tcp"
	if _, err := exec.LookPath("ufw"); err == nil {
		out, _ := exec.Command("ufw", "status").CombinedOutput()
		if strings.Contains(strings.ToLower(string(out)), "status: active") {
			if out2, err := exec.Command("ufw", "allow", p).CombinedOutput(); err != nil {
				return "UFW 自动放行失败: " + strings.TrimSpace(string(out2))
			}
		}
	}
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		if exec.Command("firewall-cmd", "--state").Run() == nil {
			if out, err := exec.Command("firewall-cmd", "--permanent", "--add-port="+p).CombinedOutput(); err != nil {
				return "firewalld 自动放行失败: " + strings.TrimSpace(string(out))
			}
			_ = exec.Command("firewall-cmd", "--reload").Run()
		}
	}
	return ""
}
