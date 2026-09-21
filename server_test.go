package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeNodeAddress(t *testing.T) {
	tests := map[string]string{
		"vpn.example.com": "vpn.example.com",
		"VPN.Example.COM.": "vpn.example.com",
		"198.35.45.175": "198.35.45.175",
		"[2001:db8::1]": "2001:db8::1",
	}
	for in, want := range tests {
		got, err := normalizeNodeAddress(in)
		if err != nil || got != want {
			t.Fatalf("normalizeNodeAddress(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "https://vpn.example.com", "vpn.example.com:443", "bad host", "localhost"} {
		if _, err := normalizeNodeAddress(bad); err == nil {
			t.Fatalf("expected invalid node address: %q", bad)
		}
	}
}

func TestNormalizeDNSName(t *testing.T) {
	if got, err := normalizeDNSName("www.apple.com"); err != nil || got != "www.apple.com" {
		t.Fatalf("unexpected apple DNS result: %q, %v", got, err)
	}
	for _, bad := range []string{"www.apple.com/path", "-apple.com", "apple-.com", "apple", "apple_com"} {
		if _, err := normalizeDNSName(bad); err == nil {
			t.Fatalf("expected invalid DNS name: %q", bad)
		}
	}
}

func TestValidateEditedPortRejectsOtherNode(t *testing.T) {
	nodes := []Node{{ID: "a", Port: 20001}, {ID: "b", Port: 20002}}
	if err := validateEditedPort(20002, "a", nodes); err == nil {
		t.Fatal("expected duplicate node port to be rejected")
	}
}

func TestNormalizeSubscriptionSlug(t *testing.T) {
	tests := map[string]string{
		"abcd": "abcd",
		"/sub/my-sub_01/": "my-sub_01",
	}
	for in, want := range tests {
		got, err := normalizeSubscriptionSlug(in)
		if err != nil || got != want {
			t.Fatalf("normalizeSubscriptionSlug(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"a", "bad/path", "含中文"} {
		if _, err := normalizeSubscriptionSlug(bad); err == nil {
			t.Fatalf("expected invalid subscription slug: %q", bad)
		}
	}
}

func TestValidateEditedPortRejectsDigitFour(t *testing.T) {
	if err := validateEditedPort(16804, "a", nil); err == nil {
		t.Fatal("expected digit 4 in edited port to be rejected")
	}
}

func TestPanelPortDefault(t *testing.T) {
	if got := panelPort("0.0.0.0:16800"); got != "16800" {
		t.Fatalf("unexpected panel port: %s", got)
	}
}

func TestInvalidCertificateNeverForcesHTTPS(t *testing.T) {
	if certificateValidForDomain("/does/not/exist.pem", "/does/not/exist.key", "panel.example.com", time.Now()) {
		t.Fatal("missing certificate must never be treated as valid HTTPS")
	}
}

func TestPanelRestartRejectsNonPost(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/system/restart", nil)
	w := httptest.NewRecorder()
	s.handlePanelRestart(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestConfiguredPanelPortFromService(t *testing.T) {
	content := "[Service]\nEnvironment=MUI_ADDR=0.0.0.0:18080\nExecStart=/usr/local/bin/m-ui-server\n"
	if got := configuredPanelPortFromService(content, 16800); got != 18080 {
		t.Fatalf("unexpected configured panel port: %d", got)
	}
	if got := configuredPanelPortFromService("[Service]\n", 16800); got != 16800 {
		t.Fatalf("fallback port mismatch: %d", got)
	}
}

func TestReplaceServicePanelPort(t *testing.T) {
	content := "[Service]\nEnvironment=MUI_ADDR=0.0.0.0:16800\nExecStart=/usr/local/bin/m-ui-server\n"
	got, err := replaceServicePanelPort(content, 18080)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Environment=MUI_ADDR=0.0.0.0:18080") {
		t.Fatalf("port was not replaced: %s", got)
	}
	if _, err := replaceServicePanelPort(content, 80); err == nil {
		t.Fatal("expected low panel port rejection")
	}
}

func TestTrafficDeltasMapByNodeID(t *testing.T) {
	st := AppState{Nodes: []Node{{ID: "node-a", Name: "US01"}, {ID: "node-b", Name: "JP01"}}}
	raw := map[string]TrafficDelta{
		"node-a@m-ui": {Uplink: 120, Downlink: 230},
		"node-b@m-ui": {Uplink: 5, Downlink: 9},
	}
	got := trafficDeltasByNodeID(st, raw)
	if got["node-a"].Uplink != 120 || got["node-a"].Downlink != 230 {
		t.Fatalf("unexpected node-a traffic: %#v", got["node-a"])
	}
	applyNodeTrafficDeltas(&st, got)
	if st.Nodes[1].UplinkBytes != 5 || st.Nodes[1].DownlinkBytes != 9 {
		t.Fatalf("traffic was not applied: %#v", st.Nodes[1])
	}
}
