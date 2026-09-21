package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"strconv"
	"testing"
)

func TestValidationTempPathIsJSON(t *testing.T) {
	x := NewXrayManager("/usr/local/bin/xray", "/usr/local/etc/xray/config.json")
	if got := x.validationTempPath(); !strings.HasSuffix(got, ".json") {
		t.Fatalf("validation temp path must end in .json, got %q", got)
	}
}

func TestBuildTLSConfigAndURI(t *testing.T) {
	d := t.TempDir()
	cert := filepath.Join(d, "fullchain.pem")
	key := filepath.Join(d, "privkey.pem")
	if err := os.WriteFile(cert, []byte("dummy"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("dummy"), 0600); err != nil {
		t.Fatal(err)
	}
	st := defaultState()
	st.Settings.Domain = "vpn.example.com"
	st.Settings.CertFile = cert
	st.Settings.KeyFile = key
	st.Nodes = []Node{{
		ID: "n1", Name: "US01", UUID: "9e7c8e21-026e-41a6-a8f1-168a7e20d819",
		Port: 16888, Enabled: true, Security: "tls",
	}}
	x := NewXrayManager("/usr/local/bin/xray", filepath.Join(d, "config.json"))
	b, err := x.BuildConfig(st)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "xtls-rprx-vision") || !strings.Contains(s, cert) || !strings.Contains(s, `"method": "raw"`) {
		t.Fatalf("unexpected TLS config: %s", s)
	}
	uri := nodeURI(st, st.Nodes[0])
	if !strings.Contains(uri, "vless://") || !strings.Contains(uri, "security=tls") || !strings.Contains(uri, "flow=xtls-rprx-vision") {
		t.Fatalf("unexpected TLS uri: %s", uri)
	}
}

func TestBuildRealityConfigWithoutDomainOrCertificate(t *testing.T) {
	st := defaultState()
	st.Settings.Domain = ""
	st.Settings.CertFile = "/does/not/exist"
	st.Settings.KeyFile = "/does/not/exist"
	st.Nodes = []Node{{
		ID: "n1", Name: "US01", UUID: "9e7c8e21-026e-41a6-a8f1-168a7e20d819",
		Port: 16888, Enabled: true, Security: "reality", Address: "203.0.113.10",
		RealityPrivateKey: "server-private-key", RealityPassword: "client-password",
		RealityShortID: "0123456789abcdef", RealityServerName: "www.microsoft.com",
		RealityTarget: "www.microsoft.com:443",
	}}
	x := NewXrayManager("/usr/local/bin/xray", filepath.Join(t.TempDir(), "config.json"))
	b, err := x.BuildConfig(st)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"method": "raw"`, `"security": "reality"`, `"target": "www.microsoft.com:443"`, `"shortIds"`, `"minClientVer": "1.8.2"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("Reality config missing %s: %s", want, s)
		}
	}
	uri := nodeURI(st, st.Nodes[0])
	for _, want := range []string{"vless://", "203.0.113.10:16888", "security=reality", "flow=xtls-rprx-vision", "pbk=client-password", "sid=0123456789abcdef"} {
		if !strings.Contains(uri, want) {
			t.Fatalf("Reality uri missing %s: %s", want, uri)
		}
	}
}

func TestRealityURIUsesConfiguredDomainAddress(t *testing.T) {
	st := defaultState()
	n := Node{
		Name: "US01", UUID: "9e7c8e21-026e-41a6-a8f1-168a7e20d819", Port: 33353,
		Enabled: true, Security: "reality", Address: "vpn.example.com",
		RealityPassword: "client-password", RealityShortID: "0123456789abcdef",
		RealityServerName: "www.apple.com",
	}
	uri := nodeURI(st, n)
	if !strings.Contains(uri, "@vpn.example.com:33353?") || !strings.Contains(uri, "sni=www.apple.com") {
		t.Fatalf("domain address or Apple SNI missing from URI: %s", uri)
	}
}

func TestRealityURIWithIPv6Address(t *testing.T) {
	st := defaultState()
	n := Node{
		Name: "US01", UUID: "9e7c8e21-026e-41a6-a8f1-168a7e20d819", Port: 16888,
		Enabled: true, Security: "reality", Address: "2001:db8::1",
		RealityPassword: "client-password", RealityShortID: "0123456789abcdef",
		RealityServerName: "www.microsoft.com",
	}
	uri := nodeURI(st, n)
	if !strings.Contains(uri, "@[2001:db8::1]:16888?") {
		t.Fatalf("IPv6 endpoint is not bracketed correctly: %s", uri)
	}
}

func TestGeneratedUUIDAndPortAvoidDigitFour(t *testing.T) {
	for i := 0; i < 20; i++ {
		id, err := generateUUID()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(id, "4") {
			t.Fatalf("generated UUID contains digit 4: %s", id)
		}
	}
	port, err := availablePort(map[int]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strconv.Itoa(port), "4") {
		t.Fatalf("generated port contains digit 4: %d", port)
	}
}

func TestGeneratedRealityConfigWithInstalledXray(t *testing.T) {
	if os.Getenv("XRAY_INTEGRATION") != "1" {
		t.Skip("set XRAY_INTEGRATION=1 to test against installed Xray")
	}
	xrayBin := "/usr/local/bin/xray"
	if _, err := os.Stat(xrayBin); err != nil {
		t.Fatalf("Xray integration requested but binary missing: %v", err)
	}

	x := NewXrayManager(xrayBin, filepath.Join(t.TempDir(), "config.json"))
	privateKey, password, shortID, err := x.GenerateRealityMaterial()
	if err != nil {
		t.Fatal(err)
	}
	if privateKey == "" || password == "" || len(shortID) != 16 {
		t.Fatalf("bad REALITY material: private=%t password=%t shortID=%q", privateKey != "", password != "", shortID)
	}
	if strings.Contains(privateKey, "4") || strings.Contains(password, "4") || strings.Contains(shortID, "4") {
		t.Fatalf("REALITY material contains digit 4")
	}

	st := defaultState()
	st.Nodes = []Node{{
		ID: "ci", Name: "US01", UUID: "9e7c8e21-026e-41a6-a8f1-168a7e20d819",
		Port: 16888, Enabled: true, Security: "reality", Address: "203.0.113.10",
		RealityPrivateKey: privateKey, RealityPassword: password, RealityShortID: shortID,
		RealityServerName: st.Settings.RealityServerName, RealityTarget: st.Settings.RealityTarget,
	}}
	b, err := x.BuildConfig(st)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(config, b, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(xrayBin, "run", "-test", "-config", config)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("current Xray rejected generated REALITY config: %v\n%s", err, out)
	}
}
