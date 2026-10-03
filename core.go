package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

type SystemInfo struct {
	OS                   string   `json:"os"`
	OSVersion            string   `json:"os_version,omitempty"`
	HardwareModel        string   `json:"hardware_model,omitempty"`
	CPUModel             string   `json:"cpu_model,omitempty"`
	Arch                 string   `json:"arch"`
	CPUThreads           int      `json:"cpu_threads"`
	MemoryBytes          uint64   `json:"memory_bytes"`
	AvailableMemoryBytes uint64   `json:"available_memory_bytes,omitempty"`
	PortableRoot         string   `json:"portable_root"`
	Version              string   `json:"version"`
	Volumes              []Volume `json:"volumes"`
	Notes                []string `json:"notes"`
}

type Volume struct {
	Path       string `json:"path"`
	Label      string `json:"label"`
	FileSystem string `json:"file_system"`
	Removable  bool   `json:"removable"`
	SizeBytes  uint64 `json:"size_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
}

func inspectSystem(root string) (SystemInfo, error) {
	memory, available, volumes, notes := platformInfo(root)
	info := SystemInfo{OS: runtime.GOOS, Arch: runtime.GOARCH, CPUThreads: runtime.NumCPU(), MemoryBytes: memory, AvailableMemoryBytes: available, PortableRoot: root, Version: version, Volumes: volumes, Notes: notes}
	hostDetails(&info)
	return info, nil
}

func inspectNetwork() (any, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	rows := []map[string]any{}
	for _, n := range interfaces {
		if n.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := n.Addrs()
		ips := []string{}
		for _, a := range addresses {
			ips = append(ips, a.String())
		}
		rows = append(rows, map[string]any{"name": n.Name, "up": n.Flags&net.FlagUp != 0, "addresses": ips})
	}
	return map[string]any{"interfaces": rows, "internet_verified": false, "note": "网卡启用不代表互联网可用；本次只读检查，不修改 DNS、代理或防火墙。"}, nil
}

type Resource struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	Size int64  `json:"size_bytes"`
	Name string `json:"name"`
}

type ResourceScan struct {
	Files     []Resource `json:"files"`
	Warnings  []string   `json:"warnings"`
	Truncated bool       `json:"truncated"`
}

func scanResources(root string) (ResourceScan, error) {
	out := ResourceScan{Files: []Resource{}, Warnings: []string{}}
	for _, kind := range []string{"models", "packages", "images", "drivers", "runtime"} {
		dir := filepath.Join(root, kind)
		if kind == "runtime" {
			var err error
			dir, err = runtimeResourcePath(root, "runtime")
			if err != nil {
				out.Warnings = append(out.Warnings, "运行时路径不可用")
				continue
			}
		}
		info, err := os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			out.Warnings = append(out.Warnings, kind+" 目录不可读")
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			out.Warnings = append(out.Warnings, kind+" 路径不是普通目录，未扫描")
			continue
		}
		seen := 0
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				out.Warnings = append(out.Warnings, "部分资源无法读取")
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			seen++
			if seen > 2500 || len(out.Files) >= 500 {
				out.Truncated = true
				return filepath.SkipAll
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			// macOS keeps metadata beside files on exFAT/FAT drives; it is not a resource.
			if strings.HasPrefix(d.Name(), "._") || d.Name() == ".DS_Store" {
				return nil
			}
			if d.IsDir() {
				rel, _ := filepath.Rel(dir, path)
				if strings.Count(rel, string(os.PathSeparator)) >= 5 {
					return filepath.SkipDir
				}
				return nil
			}
			info, e := d.Info()
			if e != nil || !info.Mode().IsRegular() {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			out.Files = append(out.Files, Resource{kind, filepath.ToSlash(rel), info.Size(), d.Name()})
			return nil
		})
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	return out, nil
}

type LocalService struct {
	Kind      string `json:"kind"`
	Model     string `json:"model"`
	Available bool   `json:"available"`
}
type ModelState struct {
	Files      []Resource     `json:"files"`
	Services   []LocalService `json:"services"`
	Ready      bool           `json:"ready"`
	Advice     string         `json:"advice"`
	Bundled    bool           `json:"bundled"`
	BundleNote string         `json:"bundle_note"`
}

var localBackends = map[string]string{"ollama": "http://127.0.0.1:11434", "llamacpp": "http://127.0.0.1:18080"}

func backendURL(kind string) (string, bool) {
	if kind == "bundled" {
		v, ok := modelAddresses.Load(kind)
		if ok {
			return v.(string), true
		}
		return "", false
	}
	v, ok := localBackends[kind]
	return v, ok
}

func localHTTP(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func localRequest(ctx context.Context, kind, method, path string, body io.Reader) *http.Request {
	base, _ := backendURL(kind)
	req, _ := http.NewRequestWithContext(ctx, method, base+path, body)
	if key, ok := modelKeys.Load(kind); ok {
		req.Header.Set("Authorization", "Bearer "+key.(string))
	}
	return req
}

func queryLocalModels(ctx context.Context, kind string) ([]LocalService, error) {
	if _, ok := backendURL(kind); !ok {
		return nil, errors.New("本盘模型尚未启动")
	}
	path := "/v1/models"
	field, nameField := "data", "id"
	if kind == "ollama" {
		path = "/api/tags"
		field, nameField = "models", "name"
	}
	req := localRequest(ctx, kind, "GET", path, nil)
	resp, err := localHTTP(3 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("本地服务尚未就绪：%d", resp.StatusCode)
	}
	var payload map[string]any
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	rows := []LocalService{}
	list, _ := payload[field].([]any)
	for _, entry := range list {
		row, _ := entry.(map[string]any)
		name, _ := row[nameField].(string)
		if name != "" {
			rows = append(rows, LocalService{kind, name, true})
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("本地服务尚未提供模型列表")
	}
	return rows, nil
}

func inspectModels(ctx context.Context, root string) (ModelState, error) {
	scan, err := scanResources(root)
	if err != nil {
		return ModelState{}, err
	}
	state := ModelState{Files: []Resource{}, Services: []LocalService{}}
	state.Bundled, state.BundleNote = bundledAI(root)
	for _, f := range scan.Files {
		if f.Kind == "models" && strings.HasSuffix(strings.ToLower(f.Path), ".gguf") {
			state.Files = append(state.Files, f)
		}
	}
	for _, kind := range []string{"ollama", "llamacpp", "bundled"} {
		rows, err := queryLocalModels(ctx, kind)
		if err == nil {
			state.Services = append(state.Services, rows...)
			state.Ready = true
		}
	}
	if state.Ready {
		state.Advice = "已有本地模型服务，可选择模型进行离线对话。速度与能力需要在这台电脑上实际验证。"
	} else if len(state.Files) > 0 {
		state.Advice = "盘内有模型权重，尚未连接推理服务。需要匹配的平台运行时；放入 GGUF 文件不等于模型已经启动。"
	} else {
		state.Advice = "尚未发现模型权重。断网时可先使用体检、资源扫描和维护向导；完整离线 AI 需要提前备好推理程序与模型包。"
	}
	return state, nil
}

type Plan struct {
	Title   string        `json:"title"`
	Summary string        `json:"summary"`
	Steps   []string      `json:"steps"`
	Action  string        `json:"suggested_action"`
	Mode    string        `json:"mode"`
	Reply   string        `json:"reply,omitempty"`
	Metrics *ModelMetrics `json:"metrics,omitempty"`
}

type ModelMetrics struct {
	ElapsedMS        int64 `json:"elapsed_ms"`
	PromptTokens     int   `json:"prompt_tokens"`
	CompletionTokens int   `json:"completion_tokens"`
}

func guide(message string) Plan {
	p := Plan{Title: "先了解这台电脑", Summary: "我会先做只读检测，确认环境后给出下一步。", Steps: []string{"读取系统、架构、内存和磁盘信息", "检查盘内资源", "将需要改动的项目列出来，由你选择执行"}, Action: "system.inspect", Mode: "guide"}
	switch {
	case containsAny(message, "格式化", "抹盘", "重装", "分区", "启动不了", "开不了机", "蓝屏"):
		p.Title = "系统与启动救援"
		p.Summary = "先确认能否进入系统，备份重要资料，再选择匹配的救援环境。此预览版提供方案，不执行分区、格式化或重装。"
		p.Steps = []string{"记录故障和目标电脑", "确认重要资料备份和目标磁盘", "准备经验证的救援镜像；Mac 使用对应恢复流程"}
		p.Action = "resources.scan"
	case containsAny(message, "丢失", "恢复文件", "抢救", "误删", "坏盘", "备份"):
		p.Title = "保护和找回文件"
		p.Summary = "先减少对故障盘的写入；恢复输出应存到另一块健康盘。"
		p.Steps = []string{"辨认源盘与目标盘，检查目标盘容量", "先镜像或只读检查故障盘", "使用合适的恢复工具，逐项核对找回的文件"}
		p.Action = "system.inspect"
	case containsAny(message, "网络", "上网", "wifi", "Wi-Fi", "DNS", "代理"):
		p.Title = "查清不能上网的原因"
		p.Summary = "先检查网卡和地址信息，再区分 DNS、代理和服务问题。"
		p.Steps = []string{"查看启用的网卡与地址", "结合故障区分网络连接、DNS、代理和目标服务", "修改前保存原配置，修复后验证同一个问题"}
		p.Action = "network.inspect"
	case containsAny(message, "模型", "离线AI", "离线 AI", "断网", "没网"):
		p.Title = "准备离线 AI"
		p.Summary = "先看本机配置、盘内模型和推理服务；资源齐全才可断网启动。"
		p.Steps = []string{"检查模型文件与本地服务", "按照可用内存、算力和磁盘空间选模型", "使用中文维护问题做真实推理测试，再决定默认模型"}
		p.Action = "models.inspect"
	case containsAny(message, "安装", "装好", "软件", "codex", "Codex", "Claude", "node", "Node"):
		p.Title = "从盘内安装工具"
		p.Summary = "先选择正确平台和完整资源包，安装完成后运行验证。当前可真实安装的是内置便携体检工具。"
		p.Steps = []string{"检测系统与架构，查看盘内资源", "确认安装位置和将做的改动", "校验资源、安装并检查实际可用性"}
		p.Action = "resources.scan"
	case containsAny(message, "慢", "卡顿", "旧电脑", "发热", "性能"):
		p.Title = "检查电脑为什么变慢"
		p.Summary = "先看内存、磁盘剩余空间和系统环境，按证据处理。"
		p.Steps = []string{"读取硬件与磁盘信息", "检查可用资源与具体症状", "只对有证据的项目做维护，保存前后结果"}
	}
	return p
}

func containsAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func maintenancePlan(ctx context.Context, root string, input map[string]any) (Plan, error) {
	message := strings.TrimSpace(str(input, "message"))
	if message == "" {
		return Plan{}, errors.New("请描述要解决的问题")
	}
	if len(message) > 8000 {
		return Plan{}, errors.New("请将问题缩短到 8000 字节以内")
	}
	p := guide(message)
	backend := str(input, "backend")
	model := str(input, "model")
	if backend == "" {
		return p, nil
	}
	_, ok := backendURL(backend)
	if !ok || model == "" {
		return Plan{}, errors.New("请选择已连接的本地模型")
	}
	state, err := inspectModels(ctx, root)
	if err != nil {
		return Plan{}, err
	}
	found := false
	for _, s := range state.Services {
		if s.Kind == backend && s.Model == model {
			found = true
			break
		}
	}
	if !found {
		return Plan{}, errors.New("所选本地模型已不可用，请刷新后重试")
	}
	sys, _ := inspectSystem(root)
	skill := "pc-triage"
	switch p.Title {
	case "系统与启动救援":
		skill = "pc-boot-recovery"
	case "保护和找回文件":
		skill = "pc-data-rescue"
	case "查清不能上网的原因":
		skill = "pc-network-care"
	case "准备离线 AI":
		skill = "pc-local-model"
	case "从盘内安装工具":
		skill = "pc-offline-install"
	}
	principles, _ := assets.ReadFile("skills/" + skill + "/SKILL.md")
	// Skills remain reusable by capable agents. This small explanation model
	// receives prose only, so executable examples cannot become user advice.
	skillText := regexp.MustCompile("`[^`]+`").ReplaceAllString(string(principles), "对应的检测按钮")
	prompt := fmt.Sprintf("你是虾盘的维护说明助手。把下方已审核步骤解释给普通用户，使用不超过三句的简洁中文。只解释这些步骤，不添加新操作，不给命令或代码，不提接口名称。不要声称已经修好或已经查出原因。\n真实检测：系统 %s，总内存 %.1f GiB。总内存不是当前空闲内存。体检按钮可读取系统、架构、总内存和磁盘剩余空间；不能读取进程、温度或 SMART，不能判断病毒。网络按钮只查看网卡与地址。\n已审核步骤：%s。\n参考维护经验：\n%s\n最后提醒用户点击页面下方的下一步按钮开始检测。", hostLabel(sys), float64(sys.MemoryBytes)/(1<<30), strings.Join(p.Steps, "；"), skillText)
	payload := map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": prompt}, {"role": "user", "content": message}}, "stream": false, "max_tokens": 384, "temperature": 0.2}
	if backend == "llamacpp" || backend == "bundled" {
		payload["chat_template_kwargs"] = map[string]bool{"enable_thinking": false}
	}
	b, _ := json.Marshal(payload)
	req := localRequest(ctx, backend, "POST", "/v1/chat/completions", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	resp, err := localHTTP(55 * time.Second).Do(req)
	if err != nil {
		return Plan{}, fmt.Errorf("本地模型响应失败：%w", err)
	}
	defer resp.Body.Close()
	var reply struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if resp.StatusCode != 200 {
		return Plan{}, fmt.Errorf("本地模型返回状态 %d；体检和维护向导仍可使用", resp.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&reply); err != nil {
		return Plan{}, err
	}
	if len(reply.Choices) == 0 || strings.TrimSpace(reply.Choices[0].Message.Content) == "" {
		return Plan{}, errors.New("模型没有返回可展示的回答，请使用维护向导或调整模型")
	}
	p.Mode = "local-model"
	p.Reply = reply.Choices[0].Message.Content
	p.Metrics = &ModelMetrics{time.Since(started).Milliseconds(), reply.Usage.PromptTokens, reply.Usage.CompletionTokens}
	return p, nil
}

func writablePath(root string, parts ...string) (string, error) {
	current := root
	for i, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "/\\:") {
			return "", errors.New("无效的便携路径")
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", errors.New("便携数据路径不能通过链接写入其他目录")
			}
			if i < len(parts)-1 && !info.IsDir() {
				return "", errors.New("便携数据目录被文件占用")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return current, nil
}

func newFileAtomic(root string, parts []string, content []byte) (string, error) {
	path, err := writablePath(root, parts...)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	if _, err = os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("目标文件已存在，不覆盖原文件")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".xiapan-*.tmp")
	if err != nil {
		return "", err
	}
	tempName := tmp.Name()
	defer os.Remove(tempName)
	if _, err = tmp.Write(content); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err = os.Rename(tempName, path); err != nil {
		return "", err
	}
	return path, nil
}

func uniqueID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

func createReport(root string) (any, error) {
	sys, err := inspectSystem(root)
	if err != nil {
		return nil, err
	}
	resources, err := scanResources(root)
	if err != nil {
		return nil, err
	}
	id := uniqueID()
	report := map[string]any{"id": id, "created_at": time.Now().Format(time.RFC3339), "system": sys, "resources": resources, "scope": "本机只读检测；未执行修复、格式化、重装或云端模型调用。"}
	content, _ := json.MarshalIndent(report, "", "  ")
	path, err := newFileAtomic(root, []string{"data", "reports", id + ".json"}, content)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(root, path)
	return map[string]any{"id": id, "path": filepath.ToSlash(rel), "report": report}, nil
}

func listReports(root string) (any, error) {
	dir, err := writablePath(root, "data", "reports")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []any{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows := []any{}
	for i := len(entries) - 1; i >= 0 && len(rows) < 50; i-- {
		e := entries[i]
		if e.Type()&os.ModeSymlink != 0 || e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() > 2<<20 {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r map[string]any
		if json.Unmarshal(b, &r) == nil {
			rows = append(rows, r)
		}
	}
	return rows, nil
}

func installTool(ctx context.Context, root, id string) (any, error) {
	if id != "portable-check" {
		return nil, errors.New("当前只支持内置便携体检工具；不会执行资源目录里的未知安装程序")
	}
	source, err := os.Executable()
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return nil, err
	}
	if len(content) > 64<<20 {
		return nil, errors.New("内置工具大小超出预期")
	}
	name := "xiapan-check"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path, err := newFileAtomic(root, []string{"data", "tools", uniqueID(), name}, content)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0700); err != nil {
		return nil, err
	}
	check, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	originalHash, copyHash := sha256.Sum256(content), sha256.Sum256(check)
	if originalHash != copyHash {
		return nil, errors.New("安装后的校验值不一致")
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(verifyCtx, path, "version")
	cmd.Dir = root
	cmd.Env, err = portableEnv(root)
	if err != nil {
		return nil, err
	}
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("工具已复制但启动验证失败：%w", err)
	}
	if strings.TrimSpace(string(output)) != version {
		return nil, errors.New("工具版本验证不通过")
	}
	rel, _ := filepath.Rel(root, path)
	return map[string]any{"package_id": id, "path": filepath.ToSlash(rel), "sha256": hex.EncodeToString(copyHash[:]), "version": version, "verified": true, "scope": "仅安装到本盘；未修改宿主软件、PATH 或注册表"}, nil
}

func portableEnv(root string) ([]string, error) {
	env := []string{}
	for _, entry := range os.Environ() {
		key := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
		if key == "TMP" || key == "TEMP" || key == "TMPDIR" {
			continue
		}
		env = append(env, entry)
	}
	temp, err := writablePath(root, "data", "tmp")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(temp, 0700); err != nil {
		return nil, err
	}
	return append(env, "TMP="+temp, "TEMP="+temp, "TMPDIR="+temp), nil
}
