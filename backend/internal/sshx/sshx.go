// Package sshx SSH 连接与指标采集：Dial 负责建连（密码/私钥），
// CollectMetrics 用一个 shell 脚本两采样算出 CPU% 与网卡速率。
package sshx

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Dial 建立 SSH 连接。authType: "password" | "key"；secret 为明文密码或 PEM 私钥。
// hostKey 校验用 InsecureIgnoreHostKey——自管工具箱场景，目标机由管理员自己登记。
func Dial(host string, port int64, username, authType, secret string) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         8 * time.Second,
	}
	switch authType {
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(secret))
		if err != nil {
			return nil, fmt.Errorf("sshx: 私钥无效（暂不支持加密私钥）: %w", err)
		}
		cfg.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	default:
		cfg.Auth = []ssh.AuthMethod{ssh.Password(secret), ssh.KeyboardInteractive(func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
			ans := make([]string, len(questions))
			for i := range ans {
				ans[i] = secret
			}
			return ans, nil
		})}
	}
	addr := net.JoinHostPort(host, strconv.FormatInt(port, 10))
	conn, err := net.DialTimeout("tcp", addr, 8*time.Second)
	if err != nil {
		return nil, fmt.Errorf("sshx: 连接 %s 失败: %w", addr, err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("sshx: SSH 握手失败: %w", err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// Metrics 一次采集的全部指标（字节/百分比原始值，展示层格式化）
type Metrics struct {
	Hostname      string
	OS            string
	Kernel        string
	UptimeSeconds int64
	CpuPercent    int64
	Load1         string
	Load5         string
	Load15        string
	MemTotal      int64
	MemAvail      int64
	MemUsed       int64
	SwapTotal     int64
	SwapUsed      int64
	DiskTotal     int64
	DiskUsed      int64
	DiskAvail     int64
	NetRxRate     int64
	NetTxRate     int64
	NetRxTotal    int64
	NetTxTotal    int64
	Disks         []DiskMount
}

type DiskMount struct {
	Mount   string
	Fs      string
	Total   int64
	Used    int64
	Avail   int64
	Percent int64
}

// 采集脚本：两帧 /proc/stat + /proc/net/dev 中间 sleep 0.5s 算速率，
// 其余一次性读取；==NAME== 段落标记用于切分解析
const metricsScript = `
echo ==CPU1==; grep '^cpu ' /proc/stat
echo ==NET1==; cat /proc/net/dev 2>/dev/null
sleep 0.5
echo ==CPU2==; grep '^cpu ' /proc/stat
echo ==NET2==; cat /proc/net/dev 2>/dev/null
echo ==MEM==; grep -E '^(MemTotal|MemAvailable|MemFree|SwapTotal|SwapFree)' /proc/meminfo
echo ==DISKS==; df -B1 -x tmpfs -x devtmpfs -x squashfs -x overlay -x efivarfs 2>/dev/null | tail -n +2
echo ==LOAD==; cat /proc/loadavg 2>/dev/null
echo ==UP==; cat /proc/uptime 2>/dev/null
echo ==OS==; (. /etc/os-release 2>/dev/null; echo "$PRETTY_NAME"); uname -r
echo ==HOST==; hostname
`

// CollectMetrics 跑采集脚本并解析；整体超时 20s
func CollectMetrics(c *ssh.Client) (Metrics, error) {
	type result struct {
		out []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		sess, err := c.NewSession()
		if err != nil {
			ch <- result{nil, fmt.Errorf("sshx: 建立会话失败: %w", err)}
			return
		}
		defer sess.Close()
		// 脚本走 stdin 喂给 sh -s：内容含单引号，拼进 sh -c '...' 会被顶断（exit 2）
		sess.Stdin = strings.NewReader(metricsScript)
		out, err := sess.Output("sh -s")
		ch <- result{out, err}
	}()
	var out []byte
	select {
	case r := <-ch:
		if r.err != nil {
			return Metrics{}, fmt.Errorf("sshx: 采集命令失败: %w", r.err)
		}
		out = r.out
	case <-time.After(20 * time.Second):
		return Metrics{}, fmt.Errorf("sshx: 采集超时")
	}
	return parseMetrics(string(out)), nil
}

func parseMetrics(text string) Metrics {
	sections := map[string]string{}
	cur := ""
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "==") && strings.HasSuffix(trimmed, "==") {
			cur = strings.Trim(trimmed, "=")
			continue
		}
		if cur != "" {
			sections[cur] += line + "\n"
		}
	}

	m := Metrics{Disks: []DiskMount{}}
	c1, c2 := cpuFields(sections["CPU1"]), cpuFields(sections["CPU2"])
	if len(c1) >= 8 && len(c2) >= 8 {
		m.CpuPercent = cpuPercent(c1, c2)
	}
	n1, n2 := netTotals(sections["NET1"]), netTotals(sections["NET2"])
	// 脚本固定 sleep 0.5s → 速率 = 差值 × 2
	m.NetRxRate = max64(n2[0]-n1[0], 0) * 2
	m.NetTxRate = max64(n2[1]-n1[1], 0) * 2
	m.NetRxTotal, m.NetTxTotal = n2[0], n2[1]
	parseMem(&m, sections["MEM"])
	parseDisks(&m, sections["DISKS"])
	if l := strings.Fields(sections["LOAD"]); len(l) >= 3 {
		m.Load1, m.Load5, m.Load15 = l[0], l[1], l[2]
	}
	if f := strings.Fields(sections["UP"]); len(f) > 0 {
		if v, err := strconv.ParseFloat(f[0], 64); err == nil {
			m.UptimeSeconds = int64(v)
		}
	}
	if lines := strings.Split(strings.TrimSpace(sections["OS"]), "\n"); len(lines) > 0 {
		m.OS = strings.TrimSpace(lines[0])
		if len(lines) > 1 {
			m.Kernel = strings.TrimSpace(lines[1])
		}
	}
	m.Hostname = strings.TrimSpace(sections["HOST"])
	return m
}

// cpuFields /proc/stat 的 cpu 行 → [user nice system idle iowait irq softirq steal]
func cpuFields(s string) []int64 {
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "cpu ") {
			continue
		}
		f := strings.Fields(line)[1:]
		vals := make([]int64, len(f))
		for i, v := range f {
			vals[i], _ = strconv.ParseInt(v, 10, 64)
		}
		return vals
	}
	return nil
}

func cpuPercent(a, b []int64) int64 {
	total := func(v []int64) int64 {
		var s int64
		for _, x := range v {
			s += x
		}
		return s
	}
	idle := func(v []int64) int64 { return v[3] + v[4] } // idle + iowait
	dt, di := total(b)-total(a), idle(b)-idle(a)
	if dt <= 0 {
		return 0
	}
	return (dt - di) * 100 / dt
}

// netTotals /proc/net/dev → [rx_bytes合计, tx_bytes合计]（跳过 lo）
func netTotals(s string) [2]int64 {
	var rx, tx int64
	for _, line := range strings.Split(s, "\n") {
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		if strings.TrimSpace(line[:i]) == "lo" {
			continue
		}
		f := strings.Fields(line[i+1:])
		if len(f) < 9 {
			continue
		}
		r, _ := strconv.ParseInt(f[0], 10, 64)
		t, _ := strconv.ParseInt(f[8], 10, 64)
		rx, tx = rx+r, tx+t
	}
	return [2]int64{rx, tx}
}

func parseMem(m *Metrics, s string) {
	kv := map[string]int64{}
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			v, _ := strconv.ParseInt(f[1], 10, 64)
			kv[strings.TrimSuffix(f[0], ":")] = v * 1024 // kB → byte
		}
	}
	m.MemTotal = kv["MemTotal"]
	m.MemAvail = kv["MemAvailable"]
	if m.MemAvail == 0 {
		m.MemAvail = kv["MemFree"]
	}
	m.MemUsed = max64(m.MemTotal-m.MemAvail, 0)
	m.SwapTotal = kv["SwapTotal"]
	m.SwapUsed = m.SwapTotal - kv["SwapFree"]
}

func parseDisks(m *Metrics, s string) {
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		d := DiskMount{Fs: f[0], Mount: f[5]}
		d.Total, _ = strconv.ParseInt(f[1], 10, 64)
		d.Used, _ = strconv.ParseInt(f[2], 10, 64)
		d.Avail, _ = strconv.ParseInt(f[3], 10, 64)
		d.Percent, _ = strconv.ParseInt(strings.TrimSuffix(f[4], "%"), 10, 64)
		m.Disks = append(m.Disks, d)
		if d.Mount == "/" {
			m.DiskTotal, m.DiskUsed, m.DiskAvail = d.Total, d.Used, d.Avail
		}
	}
	// 没有独立 / 挂载（罕见）时取第一块盘作概览
	if m.DiskTotal == 0 && len(m.Disks) > 0 {
		m.DiskTotal, m.DiskUsed, m.DiskAvail = m.Disks[0].Total, m.Disks[0].Used, m.Disks[0].Avail
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// RunScript 终端之外保留：跑一次命令返回输出（SFTP 预热/调试用）
func RunScript(c *ssh.Client, script string) ([]byte, error) {
	sess, err := c.NewSession()
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	var buf bytes.Buffer
	sess.Stdout = &buf
	sess.Stderr = &buf
	if err := sess.Run(script); err != nil {
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}
