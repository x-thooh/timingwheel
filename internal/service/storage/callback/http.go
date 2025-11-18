package callback

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/x-thooh/delay/pkg/log"
)

type Http struct {
	client *http.Client
	lg     log.Logger
}

func init() {
	RegisterAdapter("HTTPS", NewHttp())
	RegisterAdapter("HTTP", NewHttp())
}

func NewHttp() ICallback {
	return &Http{
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:      100,                                   // 最大空闲连接数
				MaxConnsPerHost:   1000,                                  // 每个主机最多连接数
				IdleConnTimeout:   90 * time.Second,                      // 空闲连接超时
				DisableKeepAlives: false,                                 // 是否禁用长连接
				TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // HTTPS 忽略证书
			},
		},
	}
}

func (h *Http) SetLogger(lg log.Logger) ICallback {
	h.lg = lg
	return h
}

func (h *Http) Request(ctx context.Context, payload *Payload) (ret string, err error) {
	start := time.Now()
	// --- 1. 打印请求参数 ---
	h.lg.Info(ctx, "HTTP Request",
		slog.String("url", payload.Url+payload.Path),
		slog.Any("schema", payload.Schema),
		slog.Any("data", payload.Data),
	)
	defer func() {
		if err != nil {
			h.lg.Info(ctx, "HTTP Response",
				slog.String("url", payload.Url+payload.Path),
				slog.String("error", err.Error()),
			)
		}
	}()

	reader, err := MapToReader(payload.Data)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s%s", payload.Url, payload.Path)
	if !strings.HasPrefix(url, "http") {
		url = fmt.Sprintf("%s://%s", strings.ToLower(payload.Schema), url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reader)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			return
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	ret = string(body)

	// --- 2. 打印响应 ---
	h.lg.Info(ctx, "HTTP Response",
		slog.String("url", url),
		slog.Int("status", resp.StatusCode),
		slog.String("body", ret),
		slog.Duration("cost", time.Since(start)),
	)

	return ret, nil
}

func (h *Http) Close(ctx context.Context) error {
	h.client.CloseIdleConnections()
	return nil
}

func MapToReader(data map[string]any) (io.Reader, error) {
	// 1. JSON 序列化
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// 2. 包装成 io.Reader
	return bytes.NewReader(b), nil
}
