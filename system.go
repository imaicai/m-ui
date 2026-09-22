package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type SystemStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryTotal   uint64  `json:"memory_total"`
	DiskPercent   float64 `json:"disk_percent"`
	DiskUsed      uint64  `json:"disk_used"`
	DiskTotal     uint64  `json:"disk_total"`
	RXBytes       uint64  `json:"rx_bytes"`
	TXBytes       uint64  `json:"tx_bytes"`
	RXRate             float64                    `json:"rx_rate"`
	TXRate             float64                    `json:"tx_rate"`
	ProxyUplinkBytes   uint64                     `json:"proxy_uplink_bytes"`
	ProxyDownlinkBytes uint64                     `json:"proxy_downlink_bytes"`
	NodeTraffic        map[string]NodeTrafficStat `json:"node_traffic,omitempty"`
	UptimeSeconds      uint64                     `json:"uptime_seconds"`
	PanelUptimeSeconds uint64                     `json:"panel_uptime_seconds"`
	XrayRunning        bool                       `json:"xray_running"`
}

type NodeTrafficStat struct {
	UplinkBytes   uint64 `json:"uplink_bytes"`
	DownlinkBytes uint64 `json:"downlink_bytes"`
}

type netSample struct {
	rx uint64
	tx uint64
	at time.Time
}

type cpuSample struct {
	idle  uint64
	total uint64
}

func readSystemStats(prevNet *netSample, prevCPU *cpuSample) (SystemStats, netSample, cpuSample) {
	var st SystemStats
	cpuValue, prevCPUValue := cpuPercent(prevCPU)
	st.CPUPercent = cpuValue
	st.MemoryUsed, st.MemoryTotal, st.MemoryPercent = memoryStats()
	st.DiskUsed, st.DiskTotal, st.DiskPercent = diskStats()
	rx, tx := networkBytes()
	now := time.Now()
	st.RXBytes, st.TXBytes = rx, tx
	if prevNet != nil && !prevNet.at.IsZero() {
		sec := now.Sub(prevNet.at).Seconds()
		if sec > 0 {
			if rx >= prevNet.rx {
				st.RXRate = float64(rx-prevNet.rx) / sec
			}
			if tx >= prevNet.tx {
				st.TXRate = float64(tx-prevNet.tx) / sec
			}
		}
	}
	st.UptimeSeconds = uptimeSeconds()
	st.XrayRunning = serviceActive("xray")
	return st, netSample{rx: rx, tx: tx, at: now}, prevCPUValue
}

func cpuPercent(prev *cpuSample) (float64, cpuSample) {
	idle, total := readCPU()
	current := cpuSample{idle: idle, total: total}
	if prev == nil {
		return 0, current
	}
	return cpuPercentBetween(*prev, current), current
}

func cpuPercentBetween(prev, current cpuSample) float64 {
	if prev.total == 0 || current.total <= prev.total || current.idle < prev.idle {
		return 0
	}
	dt := current.total - prev.total
	di := current.idle - prev.idle
	if dt == 0 || di > dt {
		return 0
	}
	return float64(dt-di) * 100 / float64(dt)
}

func readCPU() (idle, total uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0
	}
	fs := strings.Fields(sc.Text())
	// /proc/stat 的 guest/guest_nice 已包含在 user/nice 中，不能重复计入总时间。
	for i := 1; i < len(fs) && i <= 8; i++ {
		v, _ := strconv.ParseUint(fs[i], 10, 64)
		total += v
		if i == 4 || i == 5 {
			idle += v
		}
	}
	return
}

func memoryStats() (used, total uint64, percent float64) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	vals := map[string]uint64{}
	for _, ln := range strings.Split(string(b), "\n") {
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(f[1], 10, 64)
		vals[strings.TrimSuffix(f[0], ":")] = v * 1024
	}
	total = vals["MemTotal"]
	avail := vals["MemAvailable"]
	if total >= avail {
		used = total - avail
	}
	if total > 0 {
		percent = float64(used) * 100 / float64(total)
	}
	return
}

func diskStats() (used, total uint64, percent float64) {
	var fs syscall.Statfs_t
	if syscall.Statfs("/", &fs) != nil {
		return
	}
	total = fs.Blocks * uint64(fs.Bsize)
	free := fs.Bavail * uint64(fs.Bsize)
	if total >= free {
		used = total - free
	}
	if total > 0 {
		percent = float64(used) * 100 / float64(total)
	}
	return
}

func networkBytes() (rx, tx uint64) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		if name == "lo" {
			continue
		}
		fs := strings.Fields(parts[1])
		if len(fs) < 9 {
			continue
		}
		a, _ := strconv.ParseUint(fs[0], 10, 64)
		b, _ := strconv.ParseUint(fs[8], 10, 64)
		rx += a
		tx += b
	}
	return
}

func uptimeSeconds() uint64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fs := strings.Fields(string(b))
	if len(fs) == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(fs[0], 64)
	return uint64(f)
}

func serviceActive(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", name).Run() == nil
}

func fmtBytes(v uint64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%d B", v)
	}
	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(v)/float64(div), "KMGTPE"[exp])
}


type serverIdentity struct {
	IP      string
	Country string
}

func detectServerIdentity() serverIdentity {
	client := &http.Client{Timeout: 4 * time.Second}
	if id := detectCountryIS(client); validServerIdentity(id) {
		return id
	}
	if id := detectCloudflareTrace(client); validServerIdentity(id) {
		return id
	}
	return serverIdentity{}
}

func detectCountryIS(client *http.Client) serverIdentity {
	req, _ := http.NewRequest(http.MethodGet, "https://api.country.is/", nil)
	req.Header.Set("User-Agent", "m-ui/"+version)
	resp, err := client.Do(req)
	if err != nil {
		return serverIdentity{}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return serverIdentity{}
	}
	var v struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
	}
	if json.NewDecoder(resp.Body).Decode(&v) != nil {
		return serverIdentity{}
	}
	return serverIdentity{
		IP:      strings.TrimSpace(v.IP),
		Country: strings.ToUpper(strings.TrimSpace(v.Country)),
	}
}

func detectCloudflareTrace(client *http.Client) serverIdentity {
	req, _ := http.NewRequest(http.MethodGet, "https://www.cloudflare.com/cdn-cgi/trace", nil)
	req.Header.Set("User-Agent", "m-ui/"+version)
	resp, err := client.Do(req)
	if err != nil {
		return serverIdentity{}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return serverIdentity{}
	}
	var id serverIdentity
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "ip="):
			id.IP = strings.TrimSpace(strings.TrimPrefix(line, "ip="))
		case strings.HasPrefix(line, "loc="):
			id.Country = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(line, "loc=")))
		}
	}
	return id
}

func validServerIdentity(id serverIdentity) bool {
	p := net.ParseIP(strings.TrimSpace(id.IP))
	if p == nil || p.IsLoopback() || p.IsPrivate() || p.IsUnspecified() || p.IsLinkLocalUnicast() || p.IsLinkLocalMulticast() {
		return false
	}
	country := strings.ToUpper(strings.TrimSpace(id.Country))
	if len(country) != 2 || country == "XX" {
		return false
	}
	for _, r := range country {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func normalizeCountryCode(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if len(v) != 2 || v == "XX" {
		return ""
	}
	for _, r := range v {
		if r < 'A' || r > 'Z' {
			return ""
		}
	}
	return v
}

func applyNodeCountryBackfill(st *AppState, country string) bool {
	if st == nil {
		return false
	}
	country = normalizeCountryCode(country)
	if country == "" {
		return false
	}
	changed := false
	for i := range st.Nodes {
		if normalizeCountryCode(st.Nodes[i].Country) == "" {
			st.Nodes[i].Country = country
			changed = true
		}
	}
	return changed
}

func backfillNodeCountries(store *Store) error {
	if store == nil {
		return nil
	}
	st := store.Snapshot()
	needsBackfill := false
	for _, node := range st.Nodes {
		if normalizeCountryCode(node.Country) == "" {
			needsBackfill = true
			break
		}
	}
	if !needsBackfill {
		return nil
	}
	identity := detectServerIdentity()
	if !applyNodeCountryBackfill(&st, identity.Country) {
		return nil
	}
	return store.Replace(st)
}

func nextCountryNodeName(country string, nodes []Node) string {
	country = strings.ToUpper(strings.TrimSpace(country))
	if len(country) != 2 {
		country = "NODE"
	}
	max := 0
	for _, n := range nodes {
		name := strings.ToUpper(strings.TrimSpace(n.Name))
		if !strings.HasPrefix(name, country) {
			continue
		}
		suffix := strings.TrimPrefix(name, country)
		if suffix == "" {
			continue
		}
		if v, err := strconv.Atoi(suffix); err == nil && v > max {
			max = v
		}
	}
	if country == "NODE" {
		return fmt.Sprintf("NODE%02d", max+1)
	}
	return fmt.Sprintf("%s%02d", country, max+1)
}
