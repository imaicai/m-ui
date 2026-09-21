package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildConfigAndURI(t *testing.T) {
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
	st.Nodes = []Node{{ID: "n1", Name: "US01", UUID: "9e7c8e21-026e-41a6-a8f1-168a7e20d819", Port: 16888, Enabled: true}}
	x := NewXrayManager("/usr/local/bin/xray", filepath.Join(d, "config.json"))
	b, err := x.BuildConfig(st)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "xtls-rprx-vision") || !strings.Contains(s, cert) {
		t.Fatalf("unexpected config: %s", s)
	}
	uri := nodeURI(st, st.Nodes[0])
	if !strings.Contains(uri, "vless://") || !strings.Contains(uri, "security=tls") || !strings.Contains(uri, "flow=xtls-rprx-vision") {
		t.Fatalf("unexpected uri: %s", uri)
	}
}
