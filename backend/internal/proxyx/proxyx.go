package proxyx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

const (
	SourceURL = "https://proxy5.net/api/free-proxies.php"
	pageURL   = "https://proxy5.net/cn/free-proxy"
	// version window used by the site's own JS: floor(now / 30min)
	versionWindowSec = 1800
	ua               = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
)

type Remote struct {
	IP          string   `json:"ip_address"`
	Port        int      `json:"port"`
	Protocols   []string `json:"protocols"`
	Anonymity   string   `json:"anonymity"`
	Country     string   `json:"country"`
	CountryCode string   `json:"country_code"`
	CountrySlug string   `json:"country_slug"`
	Region      string   `json:"region"`
	Latency     int      `json:"latency"`
	Speed       int      `json:"download_speed"`
	Uptime      int      `json:"uptime"`
	Working     bool     `json:"is_working"`
}

type payload struct {
	CreatedAt string   `json:"created_at_utc"`
	Count     int      `json:"count"`
	Results   []Remote `json:"results"`
}

var client = newClient()

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar:     jar,
		Timeout: 90 * time.Second,
	}
}

func version() string {
	return fmt.Sprint(time.Now().Unix() / versionWindowSec)
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", pageURL)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// Fetch pulls the full proxy catalog. If the API answers with a Cloudflare
// challenge it warms up the cookie jar by loading the listing page once, then retries.
func Fetch(ctx context.Context) ([]Remote, string, error) {
	u := SourceURL + "?v=" + version()
	body, err := get(ctx, u)
	var p payload
	if err == nil {
		err = json.Unmarshal(body, &p)
	}
	if err != nil {
		if _, warmErr := get(ctx, pageURL); warmErr == nil {
			if body, err = get(ctx, u); err == nil {
				err = json.Unmarshal(body, &p)
			}
		}
	}
	if err != nil {
		return nil, "", err
	}
	out := p.Results[:0]
	for _, r := range p.Results {
		if r.IP != "" && r.Port > 0 {
			out = append(out, r)
		}
	}
	return out, p.CreatedAt, nil
}

// ---- rola-ip 源 ----

const rolaURL = "https://rola-ip.co/proxy-api/api/v1/proxies"
const rolaReferer = "https://rola-ip.co/zh/tools/free-proxy-list/"
const rolaPageSize = 500
const rolaMaxPages = 40

type rolaRow struct {
	IP        string   `json:"ip"`
	Port      int      `json:"port"`
	Protocols []string `json:"protocols"`
	Code      string   `json:"code"`
	Country   string   `json:"country"`
	Anonymity string   `json:"anonymity"`
	Latency   *float64 `json:"latency"`
	Speed     *float64 `json:"speed"`
	Uptime    *float64 `json:"uptime"`
}

type rolaPayload struct {
	Data       []rolaRow `json:"data"`
	Pagination struct {
		Page       int `json:"page"`
		Total      int `json:"total"`
		TotalPages int `json:"totalPages"`
	} `json:"pagination"`
}

func rolaAnonymity(a string) string {
	switch strings.ToLower(a) {
	case "elite":
		return "Elite"
	case "anonymous":
		return "Anonymous"
	case "transparent":
		return "Transparent"
	}
	return a
}

// FetchRola 拉取 rola-ip 免费代理列表：公开前缀 /proxy-api，500/页翻到底。
func FetchRola(ctx context.Context) ([]Remote, error) {
	out := []Remote{}
	for page := 1; page <= rolaMaxPages; page++ {
		u := fmt.Sprintf("%s?page=%d&pageSize=%d", rolaURL, page, rolaPageSize)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Referer", rolaReferer)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		var p rolaPayload
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, err
		}
		for _, r := range p.Data {
			if r.IP == "" || r.Port <= 0 {
				continue
			}
			protos := make([]string, 0, len(r.Protocols))
			for _, pr := range r.Protocols {
				protos = append(protos, strings.ToUpper(pr))
			}
			var lat, spd, upt int
			if r.Latency != nil {
				lat = int(*r.Latency)
			}
			if r.Speed != nil {
				spd = int(*r.Speed)
			}
			if r.Uptime != nil {
				upt = int(*r.Uptime)
			}
			out = append(out, Remote{
				IP: r.IP, Port: r.Port, Protocols: protos,
				Anonymity: rolaAnonymity(r.Anonymity),
				Country: r.Country, CountryCode: strings.ToUpper(r.Code),
				Region: Continent(r.Code),
				Latency: lat, Speed: spd, Uptime: upt,
			})
		}
		if page >= p.Pagination.TotalPages || len(p.Data) == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return out, nil
}

// Continent 国家码 → 大洲（与 proxy5 的 region 字段取值对齐）
func Continent(code string) string {
	if c, ok := continents[strings.ToUpper(code)]; ok {
		return c
	}
	return "Other"
}

var continents = map[string]string{
	// Asia
	"AF": "Asia", "AM": "Asia", "AZ": "Asia", "BH": "Asia", "BD": "Asia",
	"BN": "Asia", "KH": "Asia", "CN": "Asia", "HK": "Asia", "MO": "Asia",
	"TW": "Asia", "CY": "Asia", "GE": "Asia", "IN": "Asia", "ID": "Asia",
	"IR": "Asia", "IQ": "Asia", "IL": "Asia", "JP": "Asia", "JO": "Asia",
	"KZ": "Asia", "KW": "Asia", "KG": "Asia", "LA": "Asia", "LB": "Asia",
	"MY": "Asia", "MV": "Asia", "MN": "Asia", "MM": "Asia", "NP": "Asia",
	"KP": "Asia", "OM": "Asia", "PK": "Asia", "PS": "Asia", "PH": "Asia",
	"QA": "Asia", "SA": "Asia", "SG": "Asia", "KR": "Asia", "LK": "Asia",
	"SY": "Asia", "TJ": "Asia", "TH": "Asia", "TL": "Asia", "TR": "Asia",
	"TM": "Asia", "AE": "Asia", "UZ": "Asia", "VN": "Asia", "YE": "Asia",
	// Europe
	"AL": "Europe", "AD": "Europe", "AT": "Europe", "BY": "Europe", "BE": "Europe",
	"BA": "Europe", "BG": "Europe", "HR": "Europe", "CZ": "Europe", "DK": "Europe",
	"EE": "Europe", "FI": "Europe", "FR": "Europe", "DE": "Europe", "GR": "Europe",
	"HU": "Europe", "IS": "Europe", "IE": "Europe", "IT": "Europe", "LV": "Europe",
	"LI": "Europe", "LT": "Europe", "LU": "Europe", "MT": "Europe", "MD": "Europe",
	"MC": "Europe", "ME": "Europe", "NL": "Europe", "MK": "Europe", "NO": "Europe",
	"PL": "Europe", "PT": "Europe", "RO": "Europe", "RU": "Europe", "SM": "Europe",
	"RS": "Europe", "SK": "Europe", "SI": "Europe", "ES": "Europe", "SE": "Europe",
	"CH": "Europe", "UA": "Europe", "GB": "Europe", "XK": "Europe", "VA": "Europe",
	// Americas
	"AG": "Americas", "AR": "Americas", "BS": "Americas", "BB": "Americas", "BZ": "Americas",
	"BO": "Americas", "BR": "Americas", "CA": "Americas", "CL": "Americas", "CO": "Americas",
	"CR": "Americas", "CU": "Americas", "DM": "Americas", "DO": "Americas", "EC": "Americas",
	"SV": "Americas", "GD": "Americas", "GT": "Americas", "GY": "Americas", "HT": "Americas",
	"HN": "Americas", "JM": "Americas", "MX": "Americas", "NI": "Americas", "PA": "Americas",
	"PY": "Americas", "PE": "Americas", "PR": "Americas", "KN": "Americas", "LC": "Americas",
	"VC": "Americas", "SR": "Americas", "TT": "Americas", "US": "Americas", "UY": "Americas",
	"VE": "Americas", "VG": "Americas", "AI": "Americas", "AW": "Americas", "BM": "Americas",
	// Africa
	"DZ": "Africa", "AO": "Africa", "BJ": "Africa", "BW": "Africa", "BF": "Africa",
	"BI": "Africa", "CM": "Africa", "CF": "Africa", "TD": "Africa", "KM": "Africa",
	"CG": "Africa", "CD": "Africa", "CI": "Africa", "DJ": "Africa", "EG": "Africa",
	"GQ": "Africa", "ER": "Africa", "SZ": "Africa", "ET": "Africa", "GA": "Africa",
	"GM": "Africa", "GH": "Africa", "GN": "Africa", "GW": "Africa", "KE": "Africa",
	"LS": "Africa", "LR": "Africa", "LY": "Africa", "MG": "Africa", "MW": "Africa",
	"ML": "Africa", "MR": "Africa", "MU": "Africa", "MA": "Africa", "MZ": "Africa",
	"NA": "Africa", "NE": "Africa", "NG": "Africa", "RW": "Africa", "ST": "Africa",
	"SN": "Africa", "SC": "Africa", "SL": "Africa", "SO": "Africa", "ZA": "Africa",
	"SS": "Africa", "SD": "Africa", "TZ": "Africa", "TG": "Africa", "TN": "Africa",
	"UG": "Africa", "ZM": "Africa", "ZW": "Africa",
	// Pacific
	"AU": "Pacific", "FJ": "Pacific", "KI": "Pacific", "MH": "Pacific", "FM": "Pacific",
	"NR": "Pacific", "NZ": "Pacific", "PW": "Pacific", "PG": "Pacific", "WS": "Pacific",
	"SB": "Pacific", "TO": "Pacific", "TV": "Pacific", "VU": "Pacific", "GU": "Pacific",
}

// probeTarget 测活拨测目标：公网稳定 HTTP 站点
const probeHost = "www.gstatic.com"
const probePort = 80

// Check 协议感知测活：按 protocols 依次尝试 SOCKS5/SOCKS4/HTTP 握手，
// 纯 TCP 连通不算数——必须说出协议正确的应答才算活。
// 返回握手成功的协议名（socks5/socks4/http）与拨测耗时。
func Check(ctx context.Context, ip string, port int, protocols string, timeout time.Duration) (string, int, error) {
	probes := []func(context.Context, string, int, time.Duration) error{
		probeSOCKS5, probeSOCKS4, probeHTTP,
	}
	names := []string{"socks5", "socks4", "http"}
	upper := strings.ToUpper(protocols)
	order := []int{}
	for i, name := range []string{"SOCKS5", "SOCKS4", "HTTP"} {
		if strings.Contains(upper, name) {
			order = append(order, i)
		}
	}
	if len(order) == 0 {
		order = []int{0, 1, 2}
	}
	var lastErr error
	for _, i := range order {
		start := time.Now()
		if err := probes[i](ctx, ip, port, timeout); err == nil {
			return names[i], int(time.Since(start).Milliseconds()), nil
		} else {
			lastErr = err
		}
	}
	return "", 0, lastErr
}

func dial(ctx context.Context, ip string, port int, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	return d.DialContext(ctx, "tcp", net.JoinHostPort(ip, fmt.Sprint(port)))
}

func probeSOCKS5(ctx context.Context, ip string, port int, timeout time.Duration) error {
	conn, err := dial(ctx, ip, port, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))
	// greeting: VER=5, 1 method, no-auth
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return err
	}
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	if buf[0] != 0x05 || buf[1] == 0xff {
		return fmt.Errorf("socks5 greeting rejected: %x", buf)
	}
	// CONNECT domain:port
	req := append([]byte{0x05, 0x01, 0x00, 0x03, byte(len(probeHost))}, []byte(probeHost)...)
	req = append(req, byte(probePort>>8), byte(probePort))
	if _, err := conn.Write(req); err != nil {
		return err
	}
	// reply: VER REP RSV ATYP ... — 收到合法的 socks5 应答头即算活
	head := make([]byte, 5)
	if _, err := io.ReadFull(conn, head); err != nil {
		return err
	}
	if head[0] != 0x05 {
		return fmt.Errorf("bad socks5 reply: %x", head[0])
	}
	return nil
}

func probeSOCKS4(ctx context.Context, ip string, port int, timeout time.Duration) error {
	conn, err := dial(ctx, ip, port, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))
	// CONNECT 1.1.1.1:80
	req := []byte{0x04, 0x01, 0x00, 0x50, 1, 1, 1, 1, 0x00}
	if _, err := conn.Write(req); err != nil {
		return err
	}
	buf := make([]byte, 8)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	if buf[0] != 0x00 && buf[0] != 0x04 {
		return fmt.Errorf("bad socks4 reply: %x", buf[0])
	}
	return nil
}

func probeHTTP(ctx context.Context, ip string, port int, timeout time.Duration) error {
	conn, err := dial(ctx, ip, port, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))
	req := fmt.Sprintf("GET http://%s/generate_204 HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", probeHost, probeHost)
	if _, err := conn.Write([]byte(req)); err != nil {
		return err
	}
	buf := make([]byte, 5)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}
	if string(buf) != "HTTP/" {
		return fmt.Errorf("bad http reply: %q", string(buf))
	}
	return nil
}

// ZhName maps an ISO-like country code to a Chinese display name.
func ZhName(code string) string {
	if n, ok := zhNames[strings.ToUpper(code)]; ok {
		return n
	}
	return ""
}

var zhNames = map[string]string{
	"AF": "阿富汗", "AL": "阿尔巴尼亚", "DZ": "阿尔及利亚", "AD": "安道尔", "AO": "安哥拉",
	"AG": "安提瓜和巴布达", "AR": "阿根廷", "AM": "亚美尼亚", "AU": "澳大利亚", "AT": "奥地利",
	"AZ": "阿塞拜疆", "BS": "巴哈马", "BH": "巴林", "BD": "孟加拉国", "BB": "巴巴多斯",
	"BY": "白俄罗斯", "BE": "比利时", "BZ": "伯利兹", "BJ": "贝宁", "BO": "玻利维亚",
	"BA": "波黑", "BW": "博茨瓦纳", "BR": "巴西", "BN": "文莱", "BG": "保加利亚",
	"BF": "布基纳法索", "BI": "布隆迪", "KH": "柬埔寨", "CM": "喀麦隆", "CA": "加拿大",
	"CF": "中非", "TD": "乍得", "CL": "智利", "CN": "中国", "HK": "中国香港",
	"MO": "中国澳门", "TW": "中国台湾", "CO": "哥伦比亚", "KM": "科摩罗", "CG": "刚果",
	"CD": "刚果(金)", "CR": "哥斯达黎加", "CI": "科特迪瓦", "HR": "克罗地亚", "CU": "古巴",
	"CY": "塞浦路斯", "CZ": "捷克", "DK": "丹麦", "DJ": "吉布提", "DM": "多米尼克",
	"DO": "多米尼加", "EC": "厄瓜多尔", "EG": "埃及", "SV": "萨尔瓦多", "GQ": "赤道几内亚",
	"ER": "厄立特里亚", "EE": "爱沙尼亚", "SZ": "斯威士兰", "ET": "埃塞俄比亚", "FJ": "斐济",
	"FI": "芬兰", "FR": "法国", "GA": "加蓬", "GM": "冈比亚", "GE": "格鲁吉亚",
	"DE": "德国", "GH": "加纳", "GR": "希腊", "GD": "格林纳达", "GT": "危地马拉",
	"GN": "几内亚", "GW": "几内亚比绍", "GY": "圭亚那", "HT": "海地", "HN": "洪都拉斯",
	"HU": "匈牙利", "IS": "冰岛", "IN": "印度", "ID": "印度尼西亚", "IR": "伊朗",
	"IQ": "伊拉克", "IE": "爱尔兰", "IL": "以色列", "IT": "意大利", "JM": "牙买加",
	"JP": "日本", "JO": "约旦", "KZ": "哈萨克斯坦", "KE": "肯尼亚", "KI": "基里巴斯",
	"KP": "朝鲜", "KR": "韩国", "KW": "科威特", "KG": "吉尔吉斯斯坦", "LA": "老挝",
	"LV": "拉脱维亚", "LB": "黎巴嫩", "LS": "莱索托", "LR": "利比里亚", "LY": "利比亚",
	"LI": "列支敦士登", "LT": "立陶宛", "LU": "卢森堡", "MG": "马达加斯加", "MW": "马拉维",
	"MY": "马来西亚", "MV": "马尔代夫", "ML": "马里", "MT": "马耳他", "MH": "马绍尔群岛",
	"MR": "毛里塔尼亚", "MU": "毛里求斯", "MX": "墨西哥", "FM": "密克罗尼西亚", "MD": "摩尔多瓦",
	"MC": "摩纳哥", "MN": "蒙古", "ME": "黑山", "MA": "摩洛哥", "MZ": "莫桑比克",
	"MM": "缅甸", "NA": "纳米比亚", "NR": "瑙鲁", "NP": "尼泊尔", "NL": "荷兰",
	"NZ": "新西兰", "NI": "尼加拉瓜", "NE": "尼日尔", "NG": "尼日利亚", "MK": "北马其顿",
	"NO": "挪威", "OM": "阿曼", "PK": "巴基斯坦", "PW": "帕劳", "PA": "巴拿马",
	"PG": "巴布亚新几内亚", "PY": "巴拉圭", "PE": "秘鲁", "PH": "菲律宾", "PL": "波兰",
	"PT": "葡萄牙", "QA": "卡塔尔", "RO": "罗马尼亚", "RU": "俄罗斯", "RW": "卢旺达",
	"WS": "萨摩亚", "SM": "圣马力诺", "ST": "圣多美和普林西比", "SA": "沙特阿拉伯", "SN": "塞内加尔",
	"RS": "塞尔维亚", "SC": "塞舌尔", "SL": "塞拉利昂", "SG": "新加坡", "SK": "斯洛伐克",
	"SI": "斯洛文尼亚", "SB": "所罗门群岛", "SO": "索马里", "ZA": "南非", "SS": "南苏丹",
	"ES": "西班牙", "LK": "斯里兰卡", "SD": "苏丹", "SR": "苏里南", "SE": "瑞典",
	"CH": "瑞士", "SY": "叙利亚", "TJ": "塔吉克斯坦", "TZ": "坦桑尼亚", "TH": "泰国",
	"TL": "东帝汶", "TG": "多哥", "TO": "汤加", "TT": "特立尼达和多巴哥", "TN": "突尼斯",
	"TR": "土耳其", "TM": "土库曼斯坦", "TV": "图瓦卢", "UG": "乌干达", "UA": "乌克兰",
	"AE": "阿联酋", "GB": "英国", "US": "美国", "UY": "乌拉圭", "UZ": "乌兹别克斯坦",
	"VU": "瓦努阿图", "VE": "委内瑞拉", "VN": "越南", "YE": "也门", "ZM": "赞比亚",
	"ZW": "津巴布韦", "PS": "巴勒斯坦", "XK": "科索沃", "PR": "波多黎各", "VG": "英属维尔京群岛",
}
