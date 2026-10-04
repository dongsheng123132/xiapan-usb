package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type ChatMessage struct {
	ID         string `json:"id"`
	Role       string `json:"role"`
	Text       string `json:"text"`
	Action     string `json:"action,omitempty"`
	Data       any    `json:"data,omitempty"`
	ProposalID string `json:"proposal_id,omitempty"`
	Completed  bool   `json:"completed,omitempty"`
}
type Proposal struct {
	ID     string         `json:"id"`
	Action string         `json:"action"`
	Input  map[string]any `json:"input"`
}
type ChatSession struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Version     int           `json:"version"`
	Updated     string        `json:"updated_at"`
	Messages    []ChatMessage `json:"messages"`
	Pending     []Proposal    `json:"pending"`
	Engine      string        `json:"engine"`
	progressKey string
}

var chatLocks sync.Map
var chatProgress sync.Map

func publishProgress(s *ChatSession) {
	if s.progressKey == "" {
		return
	}
	b, _ := json.Marshal(s)
	chatProgress.Store(s.progressKey, b)
}

var sessionID = regexp.MustCompile(`^[0-9a-f-]{8,64}$`)

func loadChat(root, id string) (ChatSession, error) {
	if !sessionID.MatchString(id) {
		return ChatSession{}, errors.New("无效的对话编号")
	}
	p, err := writablePath(root, "data", "chat", id+".json")
	if err != nil {
		return ChatSession{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ChatSession{}, errors.New("未找到此对话")
	}
	var s ChatSession
	if len(b) > 4<<20 || json.Unmarshal(b, &s) != nil {
		return s, errors.New("对话记录损坏，已保留原文件")
	}
	s.progressKey = root + "/" + id
	return s, nil
}
func saveChat(root string, s *ChatSession) error {
	s.Updated = time.Now().Format(time.RFC3339)
	s.Version++
	b, _ := json.MarshalIndent(s, "", "  ")
	return replaceLocalFile(root, []string{"data", "chat", s.ID + ".json"}, b)
}
func addMessage(s *ChatSession, role, text string) {
	s.Messages = append(s.Messages, ChatMessage{ID: uniqueID(), Role: role, Text: text})
}
func chatAction(ctx context.Context, root, id string, input map[string]any) (any, error) {
	if id == "chat.progress" {
		chatID := str(input, "session_id")
		if !sessionID.MatchString(chatID) {
			return nil, errors.New("无效的对话编号")
		}
		if b, ok := chatProgress.Load(root + "/" + chatID); ok {
			var v ChatSession
			json.Unmarshal(b.([]byte), &v)
			return v, nil
		}
		return nil, nil
	}
	if id == "chat.start" {
		s := ChatSession{ID: uniqueID(), Title: "新对话", Messages: []ChatMessage{}, Pending: []Proposal{}, Engine: "pico"}
		if err := saveChat(root, &s); err != nil {
			return nil, err
		}
		return s, nil
	}
	if id == "chat.list" {
		dir, err := writablePath(root, "data", "chat")
		if err != nil {
			return nil, err
		}
		files, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			return []any{}, nil
		}
		if err != nil {
			return nil, err
		}
		rows := []any{}
		for i := len(files) - 1; i >= 0 && len(rows) < 60; i-- {
			f := files[i]
			if f.IsDir() || f.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			s, err := loadChat(root, strings.TrimSuffix(f.Name(), ".json"))
			if err == nil {
				rows = append(rows, map[string]any{"id": s.ID, "title": s.Title, "updated_at": s.Updated, "version": s.Version})
			}
		}
		return rows, nil
	}
	chatID := str(input, "session_id")
	lock := rootLock(&chatLocks, root+"/"+chatID)
	lock.Lock()
	defer lock.Unlock()
	s, err := loadChat(root, chatID)
	if err != nil {
		return nil, err
	}
	if id == "chat.read" {
		return s, nil
	}
	expected, ok := input["expected_version"].(float64)
	if !ok || expected != float64(s.Version) {
		return nil, errors.New("对话已经更新，请刷新后重试；未执行操作")
	}
	if id == "chat.confirm" {
		if input["confirmed"] != true {
			return nil, errors.New("请确认页面展示的操作")
		}
		proposalID := str(input, "proposal_id")
		at := -1
		for i, p := range s.Pending {
			if p.ID == proposalID {
				at = i
				break
			}
		}
		if at < 0 {
			return nil, errors.New("此操作已经完成或不属于当前对话")
		}
		p := s.Pending[at]
		if p.Action != "report.create" && p.Action != "tools.launch" {
			return nil, errors.New("此操作不能由聊天确认执行")
		}
		p.Input["confirmed"] = true
		if p.Action == "report.create" {
			p.Input["session_id"] = s.ID
		}
		r := runAction(ctx, root, p.Action, p.Input)
		if !r.OK {
			return nil, errors.New(r.Error)
		}
		s.Pending = append(s.Pending[:at], s.Pending[at+1:]...)
		for i := range s.Messages {
			if s.Messages[i].ProposalID == proposalID {
				s.Messages[i].Completed = true
			}
		}
		appendAction(&s, p.Action, r.Data)
		if p.Action == "tools.launch" {
			addMessage(&s, "assistant", "系统已接受工具启动请求。后续操作由你在该工具中完成；尚未执行清理或驱动修复。")
		} else {
			addMessage(&s, "assistant", "这一步已实际执行并验证，结果保存在本盘。")
		}
		if err = saveChat(root, &s); err != nil {
			return nil, err
		}
		return s, nil
	}
	message := strings.TrimSpace(str(input, "message"))
	if message == "" || len(message) > 8000 {
		return nil, errors.New("请输入简短的问题（最多 8000 字节）")
	}
	if len(s.Messages) == 0 {
		runes := []rune(message)
		if len(runes) > 24 {
			runes = runes[:24]
		}
		s.Title = string(runes)
	}
	addMessage(&s, "user", message)
	s.Engine = "pico"
	// Persist the question before IO. A cancelled model call must not lose it.
	if err = saveChat(root, &s); err != nil {
		return nil, err
	}
	defer chatProgress.Delete(s.progressKey)
	publishProgress(&s)
	if strings.HasPrefix(message, "/") {
		err = runSlash(ctx, root, &s, message)
	} else {
		var text string
		text, err = picoConversation(ctx, root, &s, message, requestLanguage(root, input))
		if err == nil {
			addMessage(&s, "assistant", text)
		}
	}
	if err != nil {
		addMessage(&s, "notice", err.Error()+"。可直接点击检测按钮继续排障；只有 AI 分析需要公司 API。")
	}
	if err = saveChat(root, &s); err != nil {
		return nil, err
	}
	return s, nil
}

func actionName(id string) string {
	for _, a := range actions {
		if a.ID == id {
			return a.Name
		}
	}
	return id
}
func appendAction(s *ChatSession, id string, data any) {
	s.Messages = append(s.Messages, ChatMessage{ID: uniqueID(), Role: "action", Text: actionName(id) + " · 实际执行完成", Action: id, Data: data})
	publishProgress(s)
}
func propose(s *ChatSession, id string, input map[string]any) {
	for _, p := range s.Pending {
		if p.Action == id && str(p.Input, "tool_id") == str(input, "tool_id") && str(p.Input, "package_id") == str(input, "package_id") {
			return
		}
	}
	p := Proposal{uniqueID(), id, input}
	s.Pending = append(s.Pending, p)
	text := "保存一份新的本盘体检报告"
	if id == "tools.launch" {
		text = "打开「" + builtinToolName(str(input, "tool_id")) + "」。仅启动工具，不自动清理、卸载或安装驱动。"
	}
	s.Messages = append(s.Messages, ChatMessage{ID: uniqueID(), Role: "proposal", Text: text, Action: id, ProposalID: p.ID})
	publishProgress(s)
}
func readAction(ctx context.Context, root string, s *ChatSession, id string) error {
	r := runAction(ctx, root, id, map[string]any{})
	if !r.OK {
		return errors.New(r.Error)
	}
	appendAction(s, id, r.Data)
	return nil
}
func runSlash(ctx context.Context, root string, s *ChatSession, message string) error {
	cmd := strings.Fields(message)[0]
	aliases := map[string]string{"/启动项": "startup.inspect", "/startup": "startup.inspect", "/体检": "system.inspect", "/inspect": "system.inspect", "/网络": "network.rescue", "/network": "network.rescue", "/断网": "network.rescue", "/急救": "network.rescue", "/进程": "processes.inspect", "/processes": "processes.inspect", "/驱动": "drivers.inspect", "/drivers": "drivers.inspect", "/工具": "tools.catalog", "/tools": "tools.catalog"}
	if id := aliases[cmd]; id != "" {
		return readAction(ctx, root, s, id)
	}
	if cmd == "/报告" || cmd == "/report" {
		propose(s, "report.create", map[string]any{})
		return nil
	}
	if cmd == "/打开" || cmd == "/open" {
		args := strings.Fields(message)
		if len(args) != 2 {
			return errors.New("请选择工具集合中的打开按钮")
		}
		if err := requireBuiltinTool(args[1]); err != nil {
			return err
		}
		propose(s, "tools.launch", map[string]any{"tool_id": args[1]})
		return nil
	}
	addMessage(s, "assistant", "本盘命令：/启动项 /体检 /网络 /断网 /驱动 /工具 /进程 /报告。它们调用真实维护动作，可在专业模式查看结果。这里不是任意系统命令终端。")
	return nil
}

var chatReadActions = map[string]bool{"startup.inspect": true, "system.inspect": true, "network.inspect": true, "processes.inspect": true, "reports.list": true, "drivers.inspect": true, "tools.catalog": true, "network.rescue": true}

func modelTool(ctx context.Context, root string, s *ChatSession, id string, input map[string]any) any {
	if chatReadActions[id] {
		r := runAction(ctx, root, id, input)
		if r.OK {
			appendAction(s, id, r.Data)
		}
		return r
	}
	if id == "report.create" {
		propose(s, id, map[string]any{})
		return map[string]any{"ok": true, "status": "requires_user_confirmation", "executed": false}
	}
	if id == "tools.launch" && requireBuiltinTool(str(input, "tool_id")) == nil {
		propose(s, id, map[string]any{"tool_id": str(input, "tool_id")})
		return map[string]any{"ok": true, "status": "requires_user_confirmation", "executed": false}
	}
	return map[string]any{"ok": false, "error": "该动作未开放给 AI；未执行"}
}

func modelResponseLimit(model string) int {
	// Reasoning tokens share the completion budget. The project's Flash
	// channel requires enough room for reasoning before its visible answer.
	if strings.Contains(strings.ToLower(model), "deepseek") {
		return 8192
	}
	return 1200
}

func compactEvidence(data any) any {
	b, _ := json.Marshal(data)
	var v any
	json.Unmarshal(b, &v)
	var trim func(any) any
	trim = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			// The legacy system.inspect version field is the app version,
			// not Windows/macOS/Linux's version. Disambiguate model evidence.
			if _, system := x["cpu_threads"]; system && x["os"] != nil {
				if appVersion, ok := x["version"]; ok {
					x["app_version"] = appVersion
					delete(x, "version")
				}
			}
			for k, value := range x {
				if k == "portable_root" || k == "api_key" || k == "apiKey" {
					delete(x, k)
					continue
				}
				x[k] = trim(value)
			}
		case []any:
			if len(x) > 80 {
				x = x[:80]
			}
			for i := range x {
				x[i] = trim(x[i])
			}
			return x
		case string:
			if len(x) > 16000 {
				return x[:16000]
			}
		}
		return v
	}
	return trim(v)
}
