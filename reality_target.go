package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// defaultRealityServerName is only the fallback for the REALITY camouflage
// target. New nodes normally get a target that was probed from this very
// server (see pickRealityHost). It must never be a name that Xray-core itself
// flags as a GFW blocking risk (see realityTargetRisky).
const defaultRealityServerName = "www.nvidia.com"

// realityTargetCandidates are large, globally reachable vendor sites that are
// not on Xray-core's risk list. They are only *candidates*: every one of them
// has to pass a live TLS 1.3 + h2 probe from the server before it is used.
var realityTargetCandidates = []string{
	"www.nvidia.com",
	"www.amd.com",
	"www.samsung.com",
	"www.cisco.com",
	"www.oracle.com",
	"www.ibm.com",
	"www.intel.com",
	"www.tesla.com",
	"www.sony.com",
	"www.asus.com",
	"www.hp.com",
	"www.dell.com",
	"addons.mozilla.org",
	"www.mozilla.org",
}

// legacyRealityDefaults are the former built-in defaults. Both are flagged by
// Xray-core, so the global defaults saved with them are migrated on load.
// Nodes that already exist are intentionally left alone: changing their SNI
// would instantly break every client that still uses the old share link.
var legacyRealityDefaults = []string{"www.apple.com", "www.microsoft.com"}

func isLegacyRealityDefault(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, v := range legacyRealityDefaults {
		if host == v {
			return true
		}
	}
	return false
}

// realityTargetRisky mirrors the check in Xray-core (infra/conf/transport_security.go)
// that logs: REALITY: Choosing "<name>" as the target will increase the
// likelihood of your server's IP being blocked by the GFW.
func realityTargetRisky(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimSuffix(h, ".")
	if i := strings.LastIndex(h, ":"); i >= 0 {
		if _, err := strconv.Atoi(h[i+1:]); err == nil {
			h = h[:i]
		}
	}
	for _, suffix := range []string{".ru", ".ir", ".cn"} {
		if strings.HasSuffix(h, suffix) {
			return true
		}
	}
	for _, word := range []string{"apple", "icloud", "microsoft"} {
		if strings.Contains(h, word) {
			return true
		}
	}
	return false
}

// nodeTargetRisky reports whether an existing REALITY node still uses a
// camouflage name that Xray-core warns about.
func nodeTargetRisky(n Node) bool {
	if nodeSecurity(n) != "reality" {
		return false
	}
	return realityTargetRisky(n.RealityServerName) || realityTargetRisky(n.RealityTarget)
}

type realityProbeFunc func(ctx context.Context, host string) (time.Duration, error)

// probeRealityHost measures a full TCP + TLS handshake to host:443. A target is
// only acceptable when it negotiates TLS 1.3 *and* h2, presents a certificate
// that verifies for the host name, and answers quickly. REALITY copies the
// target's handshake shape for every connection, so the target's round-trip
// time is added to every new proxied connection.
func probeRealityHost(ctx context.Context, host string) (time.Duration, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 3 * time.Second},
		Config: &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS13,
			NextProtos: []string{"h2", "http/1.1"},
		},
	}
	best := time.Duration(0)
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		start := time.Now()
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, "443"))
		if err != nil {
			lastErr = err
			continue
		}
		elapsed := time.Since(start)
		tc, ok := conn.(*tls.Conn)
		if !ok {
			_ = conn.Close()
			return 0, errors.New("不是 TLS 连接")
		}
		state := tc.ConnectionState()
		_ = conn.Close()
		if state.Version != tls.VersionTLS13 {
			return 0, errors.New("目标不支持 TLS 1.3")
		}
		if state.NegotiatedProtocol != "h2" {
			return 0, errors.New("目标不支持 HTTP/2")
		}
		if best == 0 || elapsed < best {
			best = elapsed
		}
	}
	if best == 0 {
		return 0, lastErr
	}
	return best, nil
}

type realityProbeResult struct {
	host    string
	latency time.Duration
}

// pickRealityHost probes every candidate concurrently and returns one of the
// fastest healthy ones. Choosing randomly among the few best (instead of always
// the single fastest) keeps m-ui servers from all converging on one name.
func pickRealityHost(ctx context.Context, candidates []string, probe realityProbeFunc, randInt func(n int) int) (string, time.Duration, bool) {
	results := make([]realityProbeResult, len(candidates))
	ok := make([]bool, len(candidates))
	var wg sync.WaitGroup
	for i, host := range candidates {
		if realityTargetRisky(host) {
			continue
		}
		wg.Add(1)
		go func(i int, host string) {
			defer wg.Done()
			d, err := probe(ctx, host)
			if err != nil || d <= 0 {
				return
			}
			results[i] = realityProbeResult{host: host, latency: d}
			ok[i] = true
		}(i, host)
	}
	wg.Wait()

	good := make([]realityProbeResult, 0, len(candidates))
	for i := range results {
		if ok[i] {
			good = append(good, results[i])
		}
	}
	if len(good) == 0 {
		return "", 0, false
	}
	sort.Slice(good, func(a, b int) bool {
		if good[a].latency != good[b].latency {
			return good[a].latency < good[b].latency
		}
		return good[a].host < good[b].host
	})
	limit := good[0].latency*2 + 20*time.Millisecond
	shortlist := make([]realityProbeResult, 0, 3)
	for _, r := range good {
		if len(shortlist) == 3 || r.latency > limit {
			break
		}
		shortlist = append(shortlist, r)
	}
	idx := 0
	if randInt != nil && len(shortlist) > 1 {
		idx = randInt(len(shortlist))
		if idx < 0 || idx >= len(shortlist) {
			idx = 0
		}
	}
	return shortlist[idx].host, shortlist[idx].latency, true
}

func cryptoRandInt(n int) int {
	if n <= 1 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
}

// chooseRealityTarget returns the SNI / target pair for a new node.
//
//   - If the administrator pinned a different name in the saved settings, that
//     wins and nothing is probed.
//   - Otherwise a live probe picks a healthy, fast, non-flagged candidate.
//   - If probing finds nothing (no outbound access, missing CA bundle ...), the
//     saved default is used, falling back to the built-in default when the saved
//     value is one Xray flags.
func chooseRealityTarget(settings Settings, probe realityProbeFunc) (serverName, target string) {
	pinned := strings.ToLower(strings.TrimSpace(settings.RealityServerName))
	if pinned != "" && pinned != defaultRealityServerName && !isLegacyRealityDefault(pinned) {
		return pinned, hostWithPort(settings.RealityTarget, pinned)
	}
	if probe == nil {
		probe = probeRealityHost
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	if host, latency, ok := pickRealityHost(ctx, realityTargetCandidates, probe, cryptoRandInt); ok {
		log.Printf("m-ui: REALITY 目标已自动选择 %s（握手 %d ms）", host, latency.Milliseconds())
		return host, host + ":443"
	}
	fallback := pinned
	if fallback == "" || realityTargetRisky(fallback) {
		fallback = defaultRealityServerName
	}
	log.Printf("m-ui: REALITY 目标探测未得到可用结果，使用默认 %s", fallback)
	return fallback, fallback + ":443"
}

func hostWithPort(target, host string) string {
	target = strings.TrimSpace(target)
	if target != "" && !isLegacyRealityDefault(strings.TrimSuffix(target, ":443")) {
		return target
	}
	return fmt.Sprintf("%s:443", host)
}
