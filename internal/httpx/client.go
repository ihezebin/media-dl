package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const DefaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Options 控制 HTTP 客户端行为。
type Options struct {
	Proxy   string
	Cookies []*http.Cookie
	Timeout time.Duration
}

// Client 带重试、Cookie、代理的 HTTP 客户端。
type Client struct {
	http    *http.Client
	timeout time.Duration
}

func New(opts Options) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	if len(opts.Cookies) > 0 {
		byHost := map[string][]*http.Cookie{}
		for _, c := range opts.Cookies {
			host := c.Domain
			if host == "" {
				continue
			}
			host = strings.TrimPrefix(host, ".")
			u := &url.URL{Scheme: "https", Host: host}
			byHost[u.String()] = append(byHost[u.String()], c)
		}
		for raw, cs := range byHost {
			u, _ := url.Parse(raw)
			jar.SetCookies(u, cs)
		}
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if opts.Proxy != "" {
		pu, err := url.Parse(opts.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy: %w", err)
		}
		transport.Proxy = http.ProxyURL(pu)
	}

	return &Client{
		http: &http.Client{
			Jar:       jar,
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		timeout: timeout,
	}, nil
}

func (c *Client) HTTP() *http.Client { return c.http }

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", DefaultUA)
	}
	return c.doWithRetry(req, 3)
}

func (c *Client) Get(rawURL string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(req)
}

func (c *Client) GetBytes(rawURL string, headers map[string]string) ([]byte, *http.Response, error) {
	resp, err := c.Get(rawURL, headers)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}
	return body, resp, nil
}

func (c *Client) GetString(rawURL string, headers map[string]string) (string, *http.Response, error) {
	b, resp, err := c.GetBytes(rawURL, headers)
	return string(b), resp, err
}

func (c *Client) Post(rawURL, contentType string, body []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(req)
}

func (c *Client) PostBytes(rawURL, contentType string, body []byte, headers map[string]string) ([]byte, *http.Response, error) {
	resp, err := c.Post(rawURL, contentType, body, headers)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}
	return b, resp, nil
}

func (c *Client) PostString(rawURL, contentType string, body []byte, headers map[string]string) (string, *http.Response, error) {
	b, resp, err := c.PostBytes(rawURL, contentType, body, headers)
	return string(b), resp, err
}

func (c *Client) PostForm(rawURL string, form url.Values, headers map[string]string) ([]byte, *http.Response, error) {
	return c.PostBytes(rawURL, "application/x-www-form-urlencoded", []byte(form.Encode()), headers)
}

// ResolveRedirect 跟随短链重定向，返回最终 URL。
func (c *Client) ResolveRedirect(rawURL string, headers map[string]string) (string, error) {
	noFollow := *c.http
	noFollow.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", DefaultUA)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := noFollow.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		// 再尝试完整跟随
		resp2, err := c.Get(rawURL, headers)
		if err != nil {
			return "", err
		}
		defer resp2.Body.Close()
		return resp2.Request.URL.String(), nil
	}
	ref, err := resp.Request.URL.Parse(loc)
	if err != nil {
		return loc, nil
	}
	return ref.String(), nil
}

func (c *Client) doWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Body 重放需要 GetBody；对本项目多为 GET，直接复用即可。
		r := req.Clone(context.Background())
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r.Body = body
		}
		resp, err := c.http.Do(r)
		if err != nil {
			lastErr = err
			sleepBackoff(attempt)
			continue
		}
		// 429 / 5xx / 部分 412 可重试
		if resp.StatusCode == 429 || resp.StatusCode >= 500 || (resp.StatusCode == 412 && attempt < maxRetries-1) {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			sleepBackoff(attempt)
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("request failed after retries: %w", lastErr)
}

func sleepBackoff(attempt int) {
	base := time.Duration(1<<attempt) * time.Second
	jitter := time.Duration(rand.Intn(500)) * time.Millisecond
	time.Sleep(base + jitter)
}

// ParseCookieHeader 解析 "a=1; b=2" 形式。
func ParseCookieHeader(raw, domain string) []*http.Cookie {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []*http.Cookie
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		out = append(out, &http.Cookie{
			Name:   strings.TrimSpace(kv[0]),
			Value:  strings.TrimSpace(kv[1]),
			Domain: domain,
			Path:   "/",
		})
	}
	return out
}
