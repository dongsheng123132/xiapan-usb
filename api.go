package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func serviceURL(base, path string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("云端地址配置无效")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost")) {
		return "", errors.New("云端地址需要 HTTPS")
	}
	return strings.TrimRight(base, "/") + path, nil
}

func remoteJSON(ctx context.Context, base, path, method, key string, input, output any) error {
	address, err := serviceURL(base, path)
	if err != nil {
		return err
	}
	var body io.Reader
	if input != nil {
		b, _ := json.Marshal(input)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return errors.New("无法创建云端请求")
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 48 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return errors.New("暂时无法连接所选服务；本地工具仍可使用")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		switch response.StatusCode {
		case 401:
			return errors.New("凭证未通过验证，请检查所选服务的 API Key")
		case 402, 403:
			return errors.New("云端拒绝本次请求；可能是额度不足或模型权限受限，请联系公司 API 管理员")
		case 429:
			return errors.New("云端请求较多，请稍后重试")
		default:
			return fmt.Errorf("云端返回 HTTP %d，请稍后重试", response.StatusCode)
		}
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(output); err != nil {
		return errors.New("云端响应格式异常")
	}
	return nil
}

func localHTTP(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}
