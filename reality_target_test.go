package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type httptestResult struct {
	code int
	body string
}

func doPatchNode(s *Server, id, body string) *httptestResult {
	req := httptest.NewRequest(http.MethodPatch, "/api/nodes/"+id, strings.NewReader(body))
	w := httptest.NewRecorder()
	s.handleNode(w, req)
	return &httptestResult{code: w.Code, body: w.Body.String()}
}

func TestRealityTargetRiskyMatchesXrayCoreRule(t *testing.T) {
	risky := []string{
		"www.apple.com", "www.apple.com:443", "WWW.APPLE.COM.", "gateway.icloud.com",
		"www.microsoft.com", "login.microsoftonline.com", "example.ru", "mail.example.ir",
		"www.example.cn", "shop.apple.com.cn",
	}
	for _, v := range risky {
		if !realityTargetRisky(v) {
			t.Fatalf("%q should be reported as risky", v)
		}
	}
	safe := []string{
		defaultRealityServerName, "www.nvidia.com:443", "addons.mozilla.org", "www.samsung.com", "",
	}
	for _, v := range safe {
		if realityTargetRisky(v) {
			t.Fatalf("%q should not be reported as risky", v)
		}
	}
}

func TestBuiltInRealityTargetsAreNeverRisky(t *testing.T) {
	if realityTargetRisky(defaultRealityServerName) {
		t.Fatalf("built-in default %q is flagged by Xray-core", defaultRealityServerName)
	}
	if len(realityTargetCandidates) < 8 {
		t.Fatalf("candidate pool too small to give diversity: %d", len(realityTargetCandidates))
	}
	seen := map[string]bool{}
	for _, c := range realityTargetCandidates {
		if realityTargetRisky(c) {
			t.Fatalf("candidate %q is flagged by Xray-core", c)
		}
		if seen[c] {
			t.Fatalf("duplicate candidate %q", c)
		}
		seen[c] = true
		if _, err := normalizeDNSName(c); err != nil {
			t.Fatalf("candidate %q is not a valid DNS name: %v", c, err)
		}
	}
}

func fakeProbe(latency map[string]time.Duration) realityProbeFunc {
	return func(_ context.Context, host string) (time.Duration, error) {
		d, ok := latency[host]
		if !ok {
			return 0, errors.New("unreachable")
		}
		return d, nil
	}
}

func TestPickRealityHostSkipsFailuresAndRiskyNames(t *testing.T) {
	probe := fakeProbe(map[string]time.Duration{
		"www.apple.com":  1 * time.Millisecond, // fastest, but flagged by Xray-core
		"slow.example":   400 * time.Millisecond,
		"fast.example":   30 * time.Millisecond,
		"broken.example": 0, // zero latency is treated as a failed probe
	})
	host, lat, ok := pickRealityHost(context.Background(),
		[]string{"www.apple.com", "slow.example", "fast.example", "broken.example", "down.example"}, probe, nil)
	if !ok || host != "fast.example" || lat != 30*time.Millisecond {
		t.Fatalf("unexpected pick: %q %v %v", host, lat, ok)
	}
}

func TestPickRealityHostRandomisesOnlyAmongTheFastest(t *testing.T) {
	probe := fakeProbe(map[string]time.Duration{
		"a.example": 20 * time.Millisecond,
		"b.example": 30 * time.Millisecond,
		"c.example": 40 * time.Millisecond,
		"d.example": 45 * time.Millisecond, // 4th: outside the shortlist of three
		"z.example": 900 * time.Millisecond,
	})
	cands := []string{"z.example", "d.example", "c.example", "b.example", "a.example"}
	got := map[string]bool{}
	for i := 0; i < 3; i++ {
		i := i
		host, _, ok := pickRealityHost(context.Background(), cands, probe, func(n int) int {
			if n != 3 {
				t.Fatalf("shortlist size = %d, want 3", n)
			}
			return i
		})
		if !ok {
			t.Fatal("expected a pick")
		}
		got[host] = true
	}
	for _, want := range []string{"a.example", "b.example", "c.example"} {
		if !got[want] {
			t.Fatalf("%s never picked: %v", want, got)
		}
	}
	if got["d.example"] || got["z.example"] {
		t.Fatalf("slow candidates must not be picked: %v", got)
	}
}

func TestPickRealityHostDropsCandidatesMuchSlowerThanTheBest(t *testing.T) {
	probe := fakeProbe(map[string]time.Duration{
		"near.example": 10 * time.Millisecond,
		"far.example":  200 * time.Millisecond,
	})
	for i := 0; i < 5; i++ {
		host, _, ok := pickRealityHost(context.Background(), []string{"far.example", "near.example"}, probe, func(n int) int {
			if n != 1 {
				t.Fatalf("far candidate leaked into shortlist (n=%d)", n)
			}
			return 0
		})
		if !ok || host != "near.example" {
			t.Fatalf("unexpected pick %q", host)
		}
	}
}

func TestPickRealityHostReportsFailureWhenNothingIsUsable(t *testing.T) {
	if _, _, ok := pickRealityHost(context.Background(), []string{"a.example", "www.apple.com"}, fakeProbe(nil), nil); ok {
		t.Fatal("no probe succeeded, pick must fail")
	}
}

func TestChooseRealityTargetUsesProbeResult(t *testing.T) {
	settings := defaultState().Settings
	name, target := chooseRealityTarget(settings, func(_ context.Context, host string) (time.Duration, error) {
		if host == "addons.mozilla.org" {
			return 12 * time.Millisecond, nil
		}
		return 0, errors.New("unreachable")
	})
	if name != "addons.mozilla.org" || target != "addons.mozilla.org:443" {
		t.Fatalf("unexpected target: %q %q", name, target)
	}
}

func TestChooseRealityTargetFallsBackToSafeDefaultWhenProbingFails(t *testing.T) {
	failing := func(context.Context, string) (time.Duration, error) { return 0, errors.New("offline") }

	name, target := chooseRealityTarget(defaultState().Settings, failing)
	if name != defaultRealityServerName || target != defaultRealityServerName+":443" {
		t.Fatalf("unexpected fallback: %q %q", name, target)
	}

	// A legacy flagged default must never be used as the fallback.
	legacy := defaultState().Settings
	legacy.RealityServerName, legacy.RealityTarget = "www.apple.com", "www.apple.com:443"
	name, _ = chooseRealityTarget(legacy, failing)
	if realityTargetRisky(name) {
		t.Fatalf("fell back to a flagged name: %q", name)
	}
}

func TestChooseRealityTargetRespectsAdministratorPinnedName(t *testing.T) {
	pinned := defaultState().Settings
	pinned.RealityServerName, pinned.RealityTarget = "www.sony.com", "www.sony.com:443"
	called := false
	name, target := chooseRealityTarget(pinned, func(context.Context, string) (time.Duration, error) {
		called = true
		return time.Millisecond, nil
	})
	if called {
		t.Fatal("a pinned name must not be probed or replaced")
	}
	if name != "www.sony.com" || target != "www.sony.com:443" {
		t.Fatalf("pinned name was changed: %q %q", name, target)
	}
}

func TestNodeTargetRiskyOnlyFlagsRealityNodes(t *testing.T) {
	if !nodeTargetRisky(Node{Security: "reality", RealityServerName: "www.apple.com", RealityTarget: "www.apple.com:443"}) {
		t.Fatal("existing apple REALITY node must be flagged")
	}
	if nodeTargetRisky(Node{Security: "reality", RealityServerName: "www.nvidia.com", RealityTarget: "www.nvidia.com:443"}) {
		t.Fatal("safe REALITY node flagged")
	}
	if nodeTargetRisky(Node{Security: "tls", RealityServerName: "www.apple.com"}) {
		t.Fatal("TLS nodes have no REALITY target")
	}
}

func TestMakeBootstrapExposesTargetRisk(t *testing.T) {
	st := defaultState()
	st.Nodes = []Node{
		{ID: "old", Name: "US01", Security: "reality", Enabled: true, RealityServerName: "www.apple.com", RealityTarget: "www.apple.com:443"},
		{ID: "new", Name: "US02", Security: "reality", Enabled: true, RealityServerName: "www.nvidia.com", RealityTarget: "www.nvidia.com:443"},
	}
	views := makeBootstrap(st).Nodes
	if !views[0].TargetRisk || views[1].TargetRisk {
		t.Fatalf("unexpected target_risk flags: %v %v", views[0].TargetRisk, views[1].TargetRisk)
	}
}

func TestEditingNodeToRiskyRealityNameIsRejectedButUnchangedNameIsNot(t *testing.T) {
	store := newTestStore(t)
	if err := store.Update(func(st *AppState) error {
		st.Nodes = []Node{{
			ID: "n1", Name: "US01", Security: "reality", Port: 443, Enabled: true,
			RealityServerName: "www.apple.com", RealityTarget: "www.apple.com:443",
		}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s := NewServer(store, ServerConfig{XrayBin: "/nonexistent/xray", XrayConfig: t.TempDir() + "/config.json"})

	patch := func(body string) *httptestResult {
		return doPatchNode(s, "n1", body)
	}

	r := patch(`{"reality_server_name":"www.microsoft.com"}`)
	if r.code != 400 || !strings.Contains(r.body, "封锁") {
		t.Fatalf("switching to a risky name must be refused: %d %s", r.code, r.body)
	}
	if got := store.Snapshot().Nodes[0].RealityServerName; got != "www.apple.com" {
		t.Fatalf("rejected edit changed the node: %q", got)
	}

	// The edit dialog resends the current value; that must not be reported as a risky change.
	r = patch(`{"name":"US01-renamed","reality_server_name":"www.apple.com"}`)
	if strings.Contains(r.body, "封锁") {
		t.Fatalf("unchanged name wrongly rejected as risky: %d %s", r.code, r.body)
	}
}
