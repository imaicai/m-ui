package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitPasswordAndNewDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	s, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureInitialized("admin", "strong-pass-123"); err != nil {
		t.Fatal(err)
	}
	st := s.Snapshot()
	if st.Settings.AdminUser != "admin" {
		t.Fatalf("unexpected user: %s", st.Settings.AdminUser)
	}
	if !verifyPassword("strong-pass-123", st.Settings.PasswordSalt, st.Settings.PasswordHash) {
		t.Fatal("password verification failed")
	}
	if verifyPassword("wrong", st.Settings.PasswordSalt, st.Settings.PasswordHash) {
		t.Fatal("wrong password accepted")
	}
	if len(st.Subscriptions) != 1 {
		t.Fatalf("expected default subscription")
	}
	if st.Settings.CertFile != "/maicai/ssh/m-ui/fullchain.pem" || st.Settings.KeyFile != "/maicai/ssh/m-ui/privkey.pem" {
		t.Fatalf("unexpected certificate defaults: %#v", st.Settings)
	}
	if !strings.HasPrefix(st.Settings.PanelPath, "/") || !strings.HasSuffix(st.Settings.PanelPath, "/") || st.Settings.PanelPath == "/" {
		t.Fatalf("new install should get random panel path, got %q", st.Settings.PanelPath)
	}
	if st.Settings.DefaultProtocol != "VLESS + RAW + REALITY + XTLS Vision" {
		t.Fatalf("unexpected protocol: %s", st.Settings.DefaultProtocol)
	}
}

func TestOldStateMigratesWithoutChangingExistingNodeSecurity(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	old := AppState{
		Version: 1,
		Settings: Settings{
			PanelName: "m-ui", CertFile: "/maicai/ssh/fullchain.pem", KeyFile: "/maicai/ssh/privkey.pem",
			DefaultProtocol: "VLESS + TCP + TLS + XTLS Vision", AdminUser: "admin",
		},
		Nodes: []Node{{ID: "old", Name: "US01", UUID: "u", Port: 443, Enabled: true}},
	}
	b, _ := json.Marshal(old)
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(p)
	if err != nil {
		t.Fatal(err)
	}
	st := s.Snapshot()
	if st.Version != stateVersion || st.Settings.PanelPath != "/" {
		t.Fatalf("unexpected migration: %#v", st)
	}
	if st.Nodes[0].Security != "tls" {
		t.Fatalf("existing node must remain TLS, got %q", st.Nodes[0].Security)
	}
	if st.Settings.CertFile != "/maicai/ssh/m-ui/fullchain.pem" || st.Settings.KeyFile != "/maicai/ssh/m-ui/privkey.pem" {
		t.Fatalf("old defaults not migrated")
	}
}

func TestNormalizePanelPath(t *testing.T) {
	tests := map[string]string{
		"": "/",
		"/": "/",
		"abc123": "/abc123/",
		"/abc_123-x/": "/abc_123-x/",
	}
	for in, want := range tests {
		got, err := normalizePanelPath(in)
		if err != nil || got != want {
			t.Fatalf("normalizePanelPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"/../", "/bad path/", "/中文/"} {
		if _, err := normalizePanelPath(bad); err == nil {
			t.Fatalf("expected invalid panel path: %q", bad)
		}
	}
}
