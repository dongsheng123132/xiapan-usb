package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

// One schema and one executor serve both native PicoClaw and the cloud adapter.
func maintenanceToolSchema() map[string]any {
	ids := []string{}
	for id := range chatReadActions {
		ids = append(ids, id)
	}
	ids = append(ids, "tools.install", "tools.launch", "report.create")
	sort.Strings(ids)
	return map[string]any{"type": "object", "properties": map[string]any{
		"action":     map[string]any{"type": "string", "enum": ids},
		"package_id": map[string]any{"type": "string", "enum": []string{"portable-check"}},
		"tool_id":    map[string]any{"type": "string", "enum": builtinToolIDs()},
	}, "required": []string{"action"}, "additionalProperties": false}
}

// This endpoint exists only during one native turn. The capability is never
// included in model context; calls mutate the session held by the parent, not a
// second copy in another process. HTTP JSON responses are Streamable HTTP MCP.
func startPicoMCP(ctx context.Context, root string, session *ChatSession) (map[string]any, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		listener.Close()
		return nil, nil, err
	}
	token := hex.EncodeToString(key)
	host := listener.Addr().String()
	var mu sync.Mutex
	cache := map[string]any{}
	calls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host || r.URL.Path != "/mcp" || r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "forbidden", 403)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		if ctx.Err() != nil {
			http.Error(w, "expired", 410)
			return
		}
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&request) != nil || request.JSONRPC != "2.0" {
			http.Error(w, "invalid JSON-RPC", 400)
			return
		}
		if len(request.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		rpcError := func(code int, message string) {
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": code, "message": message}})
		}
		var result any
		switch request.Method {
		case "initialize":
			var params struct {
				Version string `json:"protocolVersion"`
			}
			json.Unmarshal(request.Params, &params)
			if params.Version != "2024-11-05" && params.Version != "2025-03-26" && params.Version != "2025-06-18" && params.Version != "2025-11-25" {
				params.Version = "2025-03-26"
			}
			result = map[string]any{"protocolVersion": params.Version, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "xiapan-maintenance", "version": version}}
		case "ping":
			result = map[string]any{}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "maintenance_action", "description": "读取当前电脑和本盘资源。安装、保存报告、打开系统工具只生成待用户确认卡片；不得把申请说成执行完成。", "inputSchema": maintenanceToolSchema()}}}
		case "tools/call":
			if v, ok := cache[string(request.ID)]; ok {
				result = v
				break
			}
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			if json.Unmarshal(request.Params, &params) != nil || params.Name != "maintenance_action" || params.Arguments == nil {
				rpcError(-32602, "unknown tool or arguments")
				return
			}
			if calls >= 12 {
				rpcError(-32602, "本轮维护动作已达上限，请继续对话")
				return
			}
			calls++
			output := modelTool(ctx, root, session, str(params.Arguments, "action"), params.Arguments)
			b, err := json.Marshal(compactEvidence(output))
			if err != nil {
				rpcError(-32603, "cannot encode evidence")
				return
			}
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": string(b)}}}
			cache[string(request.ID)] = result
		default:
			rpcError(-32601, "method not found")
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 3 * time.Second}
	go server.Serve(listener)
	stop := func() { server.Close(); mu.Lock(); mu.Unlock() }
	return map[string]any{"enabled": true, "deferred": false, "type": "http", "url": "http://" + host + "/mcp", "headers": map[string]string{"Authorization": "Bearer " + token}}, stop, nil
}

func requireBuiltinTool(id string) error {
	for _, t := range builtinTools {
		if t.ID == id {
			return nil
		}
	}
	return errors.New("此工具没有受控启动适配；未执行")
}
