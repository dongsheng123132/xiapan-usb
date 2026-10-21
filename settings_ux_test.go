package main

// 模型设置体验回归：换地址清 Key 必须显式告知；HTTP 错误必须可诊断。
// 背景：用户反馈「填了第三方 API Key 不识别」。实测根因不是 Key 没发出去，
// 而是 1) 改过 API 基础地址后旧 Key 被按安全策略静默清空，测试得到 401；
// 2) 400/404 只报 "云端返回 HTTP xxx" 不带地址与响应体，漏 /v1、不支
// 持 tools、max_tokens=8192 超限三种原因长得一模一样。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newSettingsTestServer(t *testing.T, status int, body string) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	return server, &calls
}

const settingsTestToolCalls = `{"choices":[{"message":{"tool_calls":[{"function":{"name":"connection_check"}}]}}]}`

func TestModelSettingsChangedBaseBlankKeyFailsFast(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	server, calls := newSettingsTestServer(t, 200, settingsTestToolCalls)
	defer server.Close()
	save := map[string]any{"expected_revision": "initial", "source": "custom", "base_url": server.URL + "/v1", "model": "fixture", "api_key": "sk-fixture"}
	if _, err := modelSettingsAction(ctx, root, "settings.model.save", save); err != nil {
		t.Fatal(err)
	}
	s, err := selectedModel(root)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"expected_revision": s.Revision, "source": "custom", "base_url": server.URL + "/v2", "model": "fixture", "api_key": ""}
	if _, err = modelSettingsAction(ctx, root, "settings.model.test", input); err == nil || !strings.Contains(err.Error(), "重新填写 API Key") {
		t.Fatalf("换地址且未重填 Key 时应明确提示重新填写，实际：%v", err)
	}
	if *calls != 0 {
		t.Fatalf("换地址且未重填 Key 时不应发出请求，实际发出 %d 次", *calls)
	}
}

func TestModelSettingsSaveWarnsWhenKeyCleared(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	server, _ := newSettingsTestServer(t, 200, settingsTestToolCalls)
	defer server.Close()
	save := map[string]any{"expected_revision": "initial", "source": "custom", "base_url": server.URL + "/v1", "model": "fixture", "api_key": "sk-fixture"}
	if _, err := modelSettingsAction(ctx, root, "settings.model.save", save); err != nil {
		t.Fatal(err)
	}
	s, _ := selectedModel(root)
	next := map[string]any{"expected_revision": s.Revision, "source": "custom", "base_url": server.URL + "/v2", "model": "fixture", "api_key": ""}
	view, err := modelSettingsAction(ctx, root, "settings.model.save", next)
	if err != nil {
		t.Fatal(err)
	}
	v := view.(map[string]any)
	warning, _ := v["warning"].(string)
	if !strings.Contains(warning, "重新填写") || v["has_api_key"] != false {
		t.Fatalf("保存成功但应返回明确警告且 has_api_key=false，实际：%#v", v)
	}
}

func TestModelSettingsFirstTimeNoKeyStillAllowed(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	server, calls := newSettingsTestServer(t, 200, settingsTestToolCalls)
	defer server.Close()
	input := map[string]any{"expected_revision": "initial", "source": "custom", "base_url": server.URL + "/v1", "model": "fixture", "api_key": ""}
	if _, err := modelSettingsAction(ctx, root, "settings.model.test", input); err != nil {
		t.Fatalf("本机无密钥服务首次测试不应被拦截：%v", err)
	}
	if *calls != 1 {
		t.Fatalf("应发出 1 次请求，实际 %d 次", *calls)
	}
}

func TestModelSettingsHTTP404ExplainsBaseURL(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	server, _ := newSettingsTestServer(t, 404, `{"error":{"message":"unknown path /chat/completions"}}`)
	defer server.Close()
	input := map[string]any{"expected_revision": "initial", "source": "custom", "base_url": server.URL, "model": "fixture", "api_key": "sk-fixture"}
	_, err := modelSettingsAction(ctx, root, "settings.model.test", input)
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") || !strings.Contains(err.Error(), "基础地址") || !strings.Contains(err.Error(), "/v1") {
		t.Fatalf("404 错误必须提示基础地址/是否带 /v1，实际：%v", err)
	}
}

func TestModelSettingsHTTP400ExplainsToolsAndLimit(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	server, _ := newSettingsTestServer(t, 400, `{"error":{"message":"tools is not supported for this model"}}`)
	defer server.Close()
	input := map[string]any{"expected_revision": "initial", "source": "custom", "base_url": server.URL + "/v1", "model": "fixture", "api_key": "sk-fixture"}
	_, err := modelSettingsAction(ctx, root, "settings.model.test", input)
	if err == nil || !strings.Contains(err.Error(), "tools/tool_choice") || !strings.Contains(err.Error(), "8192") {
		t.Fatalf("400 错误必须提示可能是工具调用或 max_tokens=8192 超限，实际：%v", err)
	}
}
