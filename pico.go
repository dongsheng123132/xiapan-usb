package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type PicoManifest struct {
	Ready        bool                `json:"ready"`
	Version      string              `json:"version"`
	Distribution string              `json:"distribution"`
	Platforms    map[string]PicoSlot `json:"platforms"`
}
type PicoSlot struct {
	Path         string `json:"path"`
	SHA          string `json:"sha256"`
	LauncherPath string `json:"launcher_path"`
	LauncherSHA  string `json:"launcher_sha256"`
}

func (m PicoManifest) currentSlot() (PicoSlot, bool) {
	if !m.Ready {
		return PicoSlot{}, false
	}
	s, ok := m.Platforms[runtime.GOOS+"/"+runtime.GOARCH]
	return s, ok
}
func init() { nativeAgent.ApplyKey = applyPicoCredential; nativeAgent.Chat = picoConversation }
func picoManifest() PicoManifest {
	b, _ := assets.ReadFile("catalog/picoclaw.json")
	var p PicoManifest
	json.Unmarshal(b, &p)
	return p
}

var picoConfigLocks sync.Map

const picoManagedModel = "xiapan-maintenance"

// The native configuration is authoritative. Only our managed provider changes
// with the portable selection or wallet key. Wallet callbacks already hold
// their lock, so use the supplied key without re-entering walletKey.
func applyPicoCredential(root, key string) error {
	selection, err := selectedModel(root)
	if err != nil {
		return err
	}
	if selection.Source == "custom" {
		key = selection.APIKey
	}
	lock := rootLock(&picoConfigLocks, root)
	lock.Lock()
	defer lock.Unlock()
	workspace, err := preparePicoWorkspace(root)
	if err != nil {
		return err
	}
	cfg, err := readPicoMap(root, "config.json", false)
	if err != nil {
		return err
	}
	security, err := readPicoMap(root, ".security.yml", true)
	if err != nil {
		return err
	}
	marker, err := readPicoMap(root, ".xiapan-bridge.json", false)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(selection.Revision + "\x00" + selection.Source + "\x00" + selection.Model + "\x00" + selection.BaseURL + "\x00" + key))
	changed := marker["credential_fingerprint"] != hex.EncodeToString(sum[:])
	if len(cfg) == 0 {
		cfg = picoConfigForModel(workspace, selection)
	}
	if len(marker) == 0 {
		defaults := picoMap(picoMap(cfg, "agents"), "defaults")
		defaults["restrict_to_workspace"] = false
		defaults["max_tool_iterations"] = 50
		for _, name := range []string{"exec", "write_file", "edit_file", "append_file", "spawn", "spawn_status", "subagent", "cron", "mcp", "skills", "install_skill", "find_skills", "web", "web_fetch", "message", "send_file", "send_tts"} {
			picoMap(picoMap(cfg, "tools"), name)["enabled"] = true
		}
	}
	if changed {
		model := picoConfigForModel(workspace, selection)["model_list"].([]any)[0]
		models, _ := cfg["model_list"].([]any)
		found := false
		for i, entry := range models {
			if m, ok := entry.(map[string]any); ok && m["model_name"] == picoManagedModel {
				models[i] = model
				found = true
				break
			}
		}
		if !found {
			models = append(models, model)
		}
		cfg["model_list"] = models
		picoMap(picoMap(cfg, "agents"), "defaults")["model_name"] = picoManagedModel
		keys := []string{}
		if key != "" {
			keys = append(keys, key)
		}
		picoMap(security, "model_list")[picoManagedModel+":0"] = map[string]any{"api_keys": keys}
		marker["credential_fingerprint"] = hex.EncodeToString(sum[:])
	}
	picoMap(picoMap(cfg, "agents"), "defaults")["workspace"] = workspace
	if err = writePicoMap(root, ".security.yml", security, true); err != nil {
		return err
	}
	if err = writePicoMap(root, "config.json", cfg, false); err != nil {
		return err
	}
	if err = writePicoMap(root, ".xiapan-bridge.json", marker, false); err != nil {
		return err
	}
	if changed {
		absolute, _ := filepath.Abs(root)
		if value, ok := picoRuntimes.Load(absolute); ok {
			value.(*picoGatewayRuntime).restartNeeded.Store(true)
		}
	}
	return nil
}
func picoMap(parent map[string]any, key string) map[string]any {
	if m, ok := parent[key].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	parent[key] = m
	return m
}
func readPicoMap(root, name string, isYAML bool) (map[string]any, error) {
	p, err := writablePath(root, "data", "pico", name)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 4*1024*1024 {
		return nil, errors.New("PicoClaw 配置超过读取上限，原文件已保留")
	}
	m := map[string]any{}
	if isYAML {
		err = yaml.Unmarshal(b, &m)
	} else {
		err = json.Unmarshal(b, &m)
	}
	if err != nil || m == nil {
		return nil, errors.New("PicoClaw 配置损坏，已保留原文件；请先恢复配置备份")
	}
	return m, nil
}
func writePicoMap(root, name string, m map[string]any, isYAML bool) error {
	var b []byte
	var err error
	if isYAML {
		b, err = yaml.Marshal(m)
	} else {
		b, err = json.MarshalIndent(m, "", "  ")
	}
	if err != nil {
		return err
	}
	p, err := writablePath(root, "data", "pico", name)
	if err != nil {
		return err
	}
	if old, e := os.ReadFile(p); e == nil && strings.TrimSpace(string(old)) == strings.TrimSpace(string(b)) {
		return nil
	}
	return replaceLocalFile(root, []string{"data", "pico", name}, b)
}
func preparePicoWorkspace(root string) (string, error) {
	workspace, err := writablePath(root, "data", "pico", "workspace")
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(workspace, 0700); err != nil {
		return "", err
	}
	for _, name := range []string{"pc-triage", "pc-network-care", "pc-offline-install", "pc-data-rescue", "pc-boot-recovery", "aigc"} {
		if err = seedPicoSkill(root, name); err != nil {
			return "", err
		}
	}
	return workspace, nil
}

// seedPicoSkill writes one embedded skill into the drive workspace: its SKILL.md
// and, for skills that ship scripts, the files under skills/<name>/scripts/.
// It never overwrites a file the user already edited.
func seedPicoSkill(root, name string) error {
	mdParts := []string{"data", "pico", "workspace", "skills", name, "SKILL.md"}
	mdPath, err := writablePath(root, mdParts...)
	if err != nil {
		return err
	}
	if _, err = os.Stat(mdPath); errors.Is(err, os.ErrNotExist) {
		b, err := assets.ReadFile("skills/" + name + "/SKILL.md")
		if err != nil {
			return err
		}
		if _, err = newFileAtomic(root, mdParts, b); err != nil {
			return err
		}
	}
	scripts, err := fs.Glob(assets, "skills/"+name+"/scripts/*")
	if err != nil {
		return err
	}
	for _, sp := range scripts {
		rel := strings.TrimPrefix(sp, "skills/"+name+"/")
		parts := append([]string{"data", "pico", "workspace", "skills", name}, strings.Split(rel, "/")...)
		p, err := writablePath(root, parts...)
		if err != nil {
			return err
		}
		if _, err = os.Stat(p); errors.Is(err, os.ErrNotExist) {
			b, err := assets.ReadFile(sp)
			if err != nil {
				return err
			}
			if _, err = newFileAtomic(root, parts, b); err != nil {
				return err
			}
		}
	}
	return nil
}
func picoConfig(workspace string) map[string]any {
	return map[string]any{"version": 3, "agents": map[string]any{"defaults": map[string]any{"workspace": workspace, "restrict_to_workspace": false, "model_name": picoManagedModel, "max_tool_iterations": 50, "max_tokens": 8192}}, "tools": map[string]any{"filter_sensitive_data": true, "filter_min_length": 1}}
}
func picoSecurity(key string) map[string]any {
	keys := []string{}
	if key != "" {
		keys = append(keys, key)
	}
	return map[string]any{"channel_list": map[string]any{}, "model_list": map[string]any{picoManagedModel + ":0": map[string]any{"api_keys": keys}}}
}
func picoConfigForModel(workspace string, selection ModelSettings) map[string]any {
	cfg := picoConfig(workspace)
	provider, base := "deepseek", cloudConfig().API+"/v1"
	if selection.Source == "custom" {
		provider, base = "openai", selection.BaseURL
	}
	picoMap(picoMap(cfg, "agents"), "defaults")["max_tokens"] = 8192
	cfg["model_list"] = []any{map[string]any{"model_name": picoManagedModel, "provider": provider, "model": selection.Model, "api_base": base, "enabled": true}}
	return cfg
}
func picoConversation(ctx context.Context, root string, session *ChatSession, message, language string) (string, error) {
	g, err := ensurePicoGateway(ctx, root, false)
	if err != nil {
		return "", err
	}
	g.activeTurns.Add(1)
	defer g.activeTurns.Add(-1)
	binding := uniqueID()
	g.bindMu.Lock()
	g.bindings[binding] = session
	g.bindMu.Unlock()
	defer func() { g.bindMu.Lock(); delete(g.bindings, binding); g.bindMu.Unlock() }()
	prompt := "你是随身 AI 工作助手。根据任务使用原生工具。电脑维护优先使用 mcp_xiapan_maintenance_action，conversation_id 为 " + binding + "；需要确认的动作只申请确认，不声称已执行。画图或作曲优先用 aigc 技能（scripts/gen-image.mjs 作图、gen-music.mjs 歌曲、gen-bgm.mjs 配乐），运行前先 node --version；没有 node 或能力不可用就明确告诉用户，不要编造或假装成功。不要读取或输出 API Key。用简洁中文回答。\n用户：" + message
	if language == "en" {
		prompt = "You are a portable AI work assistant. Use native tools as appropriate. For maintenance prefer mcp_xiapan_maintenance_action with conversation_id " + binding + ". A confirmation request is not an executed action. For image or music generation prefer the aigc skill (scripts/gen-image.mjs, gen-music.mjs, gen-bgm.mjs); run node --version first; if node or the capability is unavailable, say so clearly instead of pretending. Never read or expose API keys. Reply in concise English.\nUser: " + message
	}
	if session.WorkspaceID != "" {
		if realPath, err := workspaceProjectRealPath(root, session.WorkspaceID); err == nil {
			if language == "en" {
				prompt += "\nWorking-directory context only (not permission isolation; actual file access stays governed by the Gateway and local tools): " + realPath
			} else {
				prompt += "\n工作目录上下文（仅是定位文件用，不是权限隔离；实际文件访问仍受 Gateway 与本机工具限制）：" + realPath
			}
		}
	}
	for _, m := range session.Messages {
		if m.Role == "action" {
			b, _ := json.Marshal(compactEvidence(m.Data))
			prompt += "\nVerified maintenance result: " + string(b)
		}
	}
	return picoGatewayChat(ctx, g, session.ID, prompt)
}
func picoEnvironment(root, engine string) ([]string, error) {
	env, err := portableEnv(root)
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(env)+5)
	for _, v := range env {
		if !strings.HasPrefix(strings.ToUpper(v), "PICOCLAW_") {
			filtered = append(filtered, v)
		}
	}
	home, err := writablePath(root, "data", "pico")
	if err != nil {
		return nil, err
	}
	out := append(filtered, "PICOCLAW_HOME="+home, "PICOCLAW_CONFIG="+filepath.Join(home, "config.json"), "PICOCLAW_BINARY="+engine, "PICOCLAW_AGENTS_DEFAULTS_WORKSPACE="+filepath.Join(home, "workspace"), "PICOCLAW_GATEWAY_HOST=127.0.0.1")
	// Expose the wallet key to the aigc skill scripts (gen-image/gen-music read
	// XIAPAN_API_KEY) without ever embedding it in a prompt or config file.
	if state, err := loadWallet(root); err == nil && state.APIKey != "" {
		out = append(out, "XIAPAN_API_KEY="+state.APIKey)
	}
	return out, nil
}
