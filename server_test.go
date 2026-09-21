package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
		"abcdefghijklmnopqrst": "abcdefghijklmnopqrst",
		"/sub/my-subscription_012345/": "my-subscription_012345",
	}
	for in, want := range tests {
		got, err := normalizeSubscriptionSlug(in)
		if err != nil || got != want {
			t.Fatalf("normalizeSubscriptionSlug(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"a", "abcd", "aaaaaaaaaaaaaaaaaaaa", "abababababababababab", "bad/path", "含中文"} {
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

func TestTrafficDeltasMapByStableNodeID(t *testing.T) {
	st := defaultState()
	st.Nodes = []Node{
		{ID: "node-a", Name: "Renamed-US01", UplinkBytes: 100, DownlinkBytes: 200},
		{ID: "node-b", Name: "JP01", UplinkBytes: 300, DownlinkBytes: 400},
	}
	raw := map[string]TrafficDelta{
		"node-a@m-ui": {Uplink: 11, Downlink: 22},
		"node-b@m-ui": {Uplink: 33, Downlink: 44},
	}
	deltas := trafficDeltasByNodeID(st, raw)
	applyNodeTrafficDeltas(&st, deltas)

	if st.Nodes[0].UplinkBytes != 111 || st.Nodes[0].DownlinkBytes != 222 {
		t.Fatalf("node-a traffic mismatch: %#v", st.Nodes[0])
	}
	if st.Nodes[1].UplinkBytes != 333 || st.Nodes[1].DownlinkBytes != 444 {
		t.Fatalf("node-b traffic mismatch: %#v", st.Nodes[1])
	}
}

func TestNewServerTracksPanelStartTime(t *testing.T) {
	before := time.Now()
	s := NewServer(nil, ServerConfig{})
	if s.startedAt.IsZero() {
		t.Fatal("panel start time must be initialized")
	}
	if s.startedAt.Before(before.Add(-time.Second)) || s.startedAt.After(time.Now().Add(time.Second)) {
		t.Fatalf("unexpected panel start time: %v", s.startedAt)
	}
}

func TestNormalizeSubscriptionNodeIDsAndMembership(t *testing.T) {
	st := defaultState()
	st.Nodes = []Node{{ID: "n1"}, {ID: "n2"}}
	ids, err := normalizeSubscriptionNodeIDs(st, []string{"n2", "n1", "n2", ""})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "n2,n1" {
		t.Fatalf("unexpected normalized IDs: %#v", ids)
	}
	if _, err := normalizeSubscriptionNodeIDs(st, []string{"missing"}); err == nil {
		t.Fatal("missing node ID should be rejected")
	}
	if !subscriptionHasNode(Subscription{IsDefault: true}, "n2") {
		t.Fatal("default subscription must contain every node")
	}
	if subscriptionHasNode(Subscription{NodeIDs: []string{"n1"}}, "n2") {
		t.Fatal("custom subscription must not contain unselected node")
	}
}

func TestSubscriptionCanBeStoppedWithoutResendingNameOrSlug(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	st := defaultState()
	st.Subscriptions = []Subscription{{
		ID: "sub-one", Name: "默认订阅", Token: "token1234", Slug: "token1234", Enabled: true, IsDefault: true,
	}}
	if err := store.Replace(st); err != nil {
		t.Fatal(err)
	}

	srv := &Server{store: store}
	req := httptest.NewRequest(http.MethodPatch, "/api/subscriptions/sub-one", strings.NewReader(`{"enabled":false}`))
	w := httptest.NewRecorder()
	srv.handleSubscription(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}
	if store.Snapshot().Subscriptions[0].Enabled {
		t.Fatal("subscription should be stopped")
	}
}

func TestWriteJSONAndFlush(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSONAndFlush(w, http.StatusOK, map[string]any{"ok": true})
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", w.Code)
	}
	if w.Header().Get("Content-Length") == "" {
		t.Fatal("Content-Length must be set before Xray restart")
	}
	if !w.Flushed {
		t.Fatal("response must be flushed before Xray restart")
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}



func TestAuthRequiresCSRFForMutatingRequests(t *testing.T) {
	s := &Server{}
	token := "session-token"
	csrf := "csrf-token-1234567890"
	s.sessions.Store(token, session{Expires: time.Now().Add(time.Hour), CSRF: csrf})
	handler := s.auth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader("{}"))
	req.AddCookie(&http.Cookie{Name: "mui_session", Value: token})
	req.Header.Set("Origin", "http://example.com")
	req.Host = "example.com"
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF should be forbidden, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader("{}"))
	req.AddCookie(&http.Cookie{Name: "mui_session", Value: token})
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("X-MUI-CSRF", csrf)
	req.Host = "example.com"
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid CSRF should pass, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequestSameOriginRejectsCrossSite(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://panel.example.com/api/test", nil)
	req.Host = "panel.example.com"
	req.Header.Set("Origin", "https://evil.example")
	if requestSameOrigin(req) {
		t.Fatal("cross-origin request must be rejected")
	}
	req.RemoteAddr = "127.0.0.1:32123"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Origin", "https://panel.example.com")
	if !requestSameOrigin(req) {
		t.Fatal("same host through a trusted TLS reverse proxy should be accepted")
	}
}


func TestRequestSecureTrustsOnlyLocalProxyHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://panel.example.com/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	if !requestSecure(req) {
		t.Fatal("loopback reverse proxy HTTPS header should be trusted")
	}

	req = httptest.NewRequest(http.MethodGet, "http://panel.example.com/", nil)
	req.RemoteAddr = "203.0.113.8:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	if requestSecure(req) {
		t.Fatal("public clients must not be able to spoof HTTPS with forwarding headers")
	}
}
