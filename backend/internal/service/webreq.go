package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/cyi-cc/fun"
	"github.com/valyala/fasthttp"

	"github.com/cyi-cc/cyi-box/backend/internal/dto"
)

const (
	webReqTimeout     = 30 * time.Second
	webRespMaxBody    = 2 << 20 // 响应体截断上限 2MB
	webReqMaxBody     = 1 << 20 // 请求体上限 1MB
	webMaxRedirect    = 5
	webMaxHeaderPairs = 64
)

// fun autowired 只处理导出字段，client 用包级单例复用
var webClient = &fasthttp.Client{
	Name:               "cyi-box-webreq",
	MaxConnsPerHost:    16,
	ReadTimeout:        webReqTimeout,
	WriteTimeout:       webReqTimeout,
	MaxConnWaitTimeout: 5 * time.Second,
}

// WebSvc HTTP 请求代理（Postman 工具）：服务端代发以绕开浏览器 CORS。
// 只放行 http/https；headersJson 为 {"k":"v"} 对象。
type WebSvc struct {
	fun.Ctx
}

func (s *WebSvc) Fetch(d dto.WebFetchDto) (dto.WebFetchView, error) {
	method := strings.ToUpper(strings.TrimSpace(d.Method))
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
	default:
		return dto.WebFetchView{}, fun.Error(4001, "不支持的请求方法: "+method)
	}
	url := strings.TrimSpace(d.Url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return dto.WebFetchView{}, fun.Error(4001, "URL 必须以 http:// 或 https:// 开头")
	}
	if len(d.Body) > webReqMaxBody {
		return dto.WebFetchView{}, fun.Error(4001, "请求体超过 1MB 上限")
	}

	var headers map[string]string
	if strings.TrimSpace(d.HeadersJson) != "" {
		if err := json.Unmarshal([]byte(d.HeadersJson), &headers); err != nil {
			return dto.WebFetchView{}, fun.Error(4001, "请求头 JSON 解析失败: "+err.Error())
		}
		if len(headers) > webMaxHeaderPairs {
			return dto.WebFetchView{}, fun.Error(4001, "请求头数量超过上限")
		}
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethod(method)
	req.SetRequestURI(url)
	for k, v := range headers {
		k = strings.TrimSpace(k)
		if k == "" || strings.EqualFold(k, "Host") || strings.EqualFold(k, "Content-Length") {
			continue
		}
		req.Header.Set(k, v)
	}
	if d.Body != "" {
		req.SetBodyString(d.Body)
	}

	start := time.Now()
	err := webClient.DoRedirects(req, resp, webMaxRedirect)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return dto.WebFetchView{}, fun.Error(4002, "请求失败: "+err.Error())
	}

	view := dto.WebFetchView{
		Status:    int64(resp.StatusCode()),
		ElapsedMs: elapsed,
	}
	respHeaders := map[string]string{}
	resp.Header.VisitAll(func(k, v []byte) {
		respHeaders[string(k)] = string(v)
	})
	if b, e := json.Marshal(respHeaders); e == nil {
		view.HeadersJson = string(b)
	}
	body := resp.Body()
	if len(body) > webRespMaxBody {
		body = body[:webRespMaxBody]
		view.Truncated = true
	}
	view.Body = string(body)
	return view, nil
}
