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
				"privateKey": n.RealityPrivateKey,
				"shortIds":   []string{n.RealityShortID},
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
					"email": n.Name + "@m-ui",
				}},
				"decryption": "none",
			},
			"streamSettings": stream,
		})
	}

	cfg := map[string]any{
		"log":      map[string]any{"loglevel": "warning"},
		"inbounds": inbounds,
		"outbounds": []any{
			map[string]any{"protocol": "freedom", "tag": "direct"},
			map[string]any{"protocol": "blackhole", "tag": "blocked"},
		},
		"routing": map[string]any{
			"domainStrategy": "AsIs",
			"rules": []any{
				map[string]any{"type": "field", "protocol": []string{"bittorrent"}, "outboundTag": "blocked"},
			},
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func nodeSecurity(n Node) string {
	if strings.EqualFold(strings.TrimSpace(n.Security), "reality") {
		return "reality"
	}
	return "tls"
}

func (x *XrayManager) Apply(st AppState) error {
	b, err := x.BuildConfig(st)
	if err != nil {
		return err
	}
	if _, err := os.Stat(x.bin); err != nil {
		return fmt.Errorf("未找到 Xray: %s", x.bin)
	}
	if err := os.MkdirAll(filepath.Dir(x.configPath), 0755); err != nil {
		return err
	}
	tmp := x.configPath + ".m-ui.tmp.json"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	defer os.Remove(tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	test := exec.CommandContext(ctx, x.bin, "run", "-test", "-config", tmp)
	if out, err := test.CombinedOutput(); err != nil {
		return fmt.Errorf("Xray 配置校验失败: %s", strings.TrimSpace(string(out)))
	}

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
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32]), nil
}

func availablePort(used map[int]bool) (int, error) {
	for i := 0; i < 300; i++ {
		rb := make([]byte, 2)
		if _, err := rand.Read(rb); err != nil {
			return 0, err
		}
		p := 10000 + int(rb[0])<<8 + int(rb[1])
		p = 10000 + (p % 50000)
		if used[p] {
			continue
		}
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(p))
		if err == nil {
			_ = ln.Close()
			return p, nil
		}
	}
	return 0, errors.New("无法自动找到可用端口")
}

func (x *XrayManager) GenerateRealityMaterial() (privateKey, password, shortID string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, x.bin, "x25519").CombinedOutput()
	if err != nil {
		return "", "", "", fmt.Errorf("REALITY 密钥生成失败: %s", strings.TrimSpace(string(out)))
	}
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
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", err
	}
	return privateKey, password, hex.EncodeToString(b), nil
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
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", n.UUID, host, n.Port, q.Encode(), url.PathEscape(n.Name))
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
