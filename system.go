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
	RXRate        float64 `json:"rx_rate"`
	TXRate        float64 `json:"tx_rate"`
	UptimeSeconds uint64  `json:"uptime_seconds"`
	XrayRunning   bool    `json:"xray_running"`
}

type netSample struct {
	rx uint64
	tx uint64
	at time.Time
}

func readSystemStats(prev *netSample) (SystemStats, netSample) {
	var st SystemStats
	st.CPUPercent = cpuPercent()
	st.MemoryUsed, st.MemoryTotal, st.MemoryPercent = memoryStats()
	st.DiskUsed, st.DiskTotal, st.DiskPercent = diskStats()
	rx, tx := networkBytes()
	now := time.Now()
	st.RXBytes, st.TXBytes = rx, tx
	if prev != nil && !prev.at.IsZero() {
		sec := now.Sub(prev.at).Seconds()
		if sec > 0 {
			if rx >= prev.rx {
				st.RXRate = float64(rx-prev.rx) / sec
			}
			if tx >= prev.tx {
				st.TXRate = float64(tx-prev.tx) / sec
			}
		}
	}
	st.UptimeSeconds = uptimeSeconds()
	st.XrayRunning = serviceActive("xray")
	return st, netSample{rx: rx, tx: tx, at: now}
}

func cpuPercent() float64 {
	idle1, total1 := readCPU()
	time.Sleep(90 * time.Millisecond)
	idle2, total2 := readCPU()
	dt := total2 - total1
	di := idle2 - idle1
	if dt == 0 {
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
	for i := 1; i < len(fs); i++ {
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
	req, _ := http.NewRequest(http.MethodGet, "https://api.country.is/", nil)
	req.Header.Set("User-Agent", "m-ui/"+version)
	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var v struct {
				IP      string `json:"ip"`
				Country string `json:"country"`
			}
			if json.NewDecoder(resp.Body).Decode(&v) == nil {
				v.IP = strings.TrimSpace(v.IP)
				v.Country = strings.ToUpper(strings.TrimSpace(v.Country))
				if net.ParseIP(v.IP) != nil && len(v.Country) == 2 {
					return serverIdentity{IP: v.IP, Country: v.Country}
				}
			}
		}
	}

	ip := ""
	if out, err := exec.Command("hostname", "-I").Output(); err == nil {
		for _, field := range strings.Fields(string(out)) {
			if p := net.ParseIP(field); p != nil && !p.IsLoopback() {
				ip = field
				break
			}
		}
	}
	return serverIdentity{IP: ip, Country: "NODE"}
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
