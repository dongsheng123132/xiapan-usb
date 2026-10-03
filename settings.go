package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
)

// The user's selection is authoritative. Per-turn PicoClaw files are derived
// from this portable state; credentials never enter action responses.
type ModelSettings struct {
	Revision string `json:"revision"`
	Source   string `json:"source"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key,omitempty"`
}

var settingsLocks sync.Map

func loadModelSettings(root string) (ModelSettings, error) {
	s := ModelSettings{Revision: "initial", Source: "custom"}
	p, err := writablePath(root, "data", "settings", "model.json")
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, errors.New("无法读取本盘模型设置")
	}
	if len(b) > 65536 || json.Unmarshal(b, &s) != nil {
		return s, errors.New("模型设置损坏，已保留原文件，请从本盘备份恢复")
	}
	if s.Source == "cloud" {
		s.Source, s.Model, s.BaseURL, s.APIKey = "custom", "", "", ""
		return s, nil
	}
	return s, validateModelSettings(s)
}

func validateModelSettings(s ModelSettings) error {
	if s.Source != "custom" {
		return errors.New("仅支持公司提供的 OpenAI 兼容 API")
	}
	if s.Model == "" || len(s.Model) > 200 || strings.ContainsAny(s.Model, "\r\n\x00") {
		return errors.New("请填写有效模型 ID")
	}
	if len(s.APIKey) > 8192 || strings.ContainsAny(s.APIKey, "\r\n\x00") {
		return errors.New("API Key 格式无效")
	}
	if s.Source == "custom" {
		if _, err := serviceURL(s.BaseURL, ""); err != nil {
			return errors.New("服务地址须为 HTTPS，或本机 localhost / 127.0.0.1 的 HTTP 地址")
		}
		u, _ := url.Parse(s.BaseURL)
		if strings.HasSuffix(u.Path, "/chat/completions") {
			return errors.New("请填写 API 基础地址（通常以 /v1 结尾），不要包含 /chat/completions")
		}
	}
	return nil
}

func settingsView(s ModelSettings) map[string]any {
	return map[string]any{"revision": s.Revision, "source": s.Source, "model": s.Model, "base_url": s.BaseURL, "has_api_key": s.APIKey != "", "portable": true}
}

func modelSettingsAction(ctx context.Context, root, id string, input map[string]any) (any, error) {
	lock := rootLock(&settingsLocks, root)
	lock.Lock()
	s, err := loadModelSettings(root)
	if err != nil {
		lock.Unlock()
		return nil, err
	}
	if id == "settings.model.get" {
		lock.Unlock()
		return settingsView(s), nil
	}
	if str(input, "expected_revision") != s.Revision {
		lock.Unlock()
		return nil, errors.New("模型设置已变化，请重新打开设置后再保存或测试")
	}
	next := s
	next.Source = str(input, "source")
	next.Model = strings.TrimSpace(str(input, "model"))
	next.BaseURL = strings.TrimRight(strings.TrimSpace(str(input, "base_url")), "/")
	if next.Source == "custom" {
		// Never silently send a saved credential to a newly entered endpoint.
		if next.BaseURL != s.BaseURL || s.Source != "custom" {
			next.APIKey = ""
		}
		if key := str(input, "api_key"); key != "" {
			next.APIKey = strings.TrimSpace(key)
		}
		if input["clear_api_key"] == true {
			next.APIKey = ""
		}
	}
	if err = validateModelSettings(next); err != nil {
		lock.Unlock()
		return nil, err
	}
	if id == "settings.model.save" {
		next.Revision = uniqueID()
		b, _ := json.MarshalIndent(next, "", "  ")
		err = replaceLocalFile(root, []string{"data", "settings", "model.json"}, b)
		lock.Unlock()
		if err != nil {
			return nil, err
		}
		return settingsView(next), nil
	}
	lock.Unlock()
	if id != "settings.model.test" {
		return nil, errors.New("未知模型设置动作")
	}
	key, base := next.APIKey, next.BaseURL
	var response struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	inputBody := map[string]any{"model": next.Model, "messages": []map[string]string{{"role": "user", "content": "Call connection_check once. This is a connection test."}}, "max_tokens": modelResponseLimit(next.Model), "stream": false, "tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": "connection_check", "description": "Test the connection without performing any operation", "parameters": map[string]any{"type": "object", "properties": map[string]any{}}}}}}
	if err = remoteJSON(ctx, base, "/chat/completions", "POST", key, inputBody, &response); err != nil {
		return nil, fmt.Errorf("模型连接测试未通过：%w", err)
	}
	for _, c := range response.Choices {
		for _, call := range c.Message.ToolCalls {
			if call.Function.Name == "connection_check" {
				return map[string]any{"connected": true, "tools_supported": true, "model": next.Model, "note": "模型连接与工具调用测试通过；尚未保存设置。"}, nil
			}
		}
	}
	return nil, errors.New("服务已响应，但模型未完成工具调用测试；请检查模型是否支持 function calling")
}

func selectedModel(root string) (ModelSettings, error) {
	lock := rootLock(&settingsLocks, root)
	lock.Lock()
	defer lock.Unlock()
	return loadModelSettings(root)
}
