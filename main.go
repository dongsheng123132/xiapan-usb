package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const version = "0.5.0-dev"

//go:embed web/* skills/*/SKILL.md catalog/*.json
var assets embed.FS

type Result struct {
	OK     bool   `json:"ok"`
	Action string `json:"action"`
	Data   any    `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Action struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Writes      bool   `json:"writes"`
}

var actions = []Action{
	{"settings.ui.get", "界面语言", "读取随盘保存的界面语言", false},
	{"settings.ui.save", "保存界面语言", "保存中英文选择，不修改模型或对话", true},
	{"settings.model.get", "模型设置", "读取本盘模型选择，不返回密钥", false},
	{"settings.model.save", "保存模型设置", "保存公司提供的 API 设置到本盘", true},
	{"settings.model.test", "测试模型连接", "发送一次工具调用测试，不保存设置", false},
	{"startup.inspect", "查看启动项", "读取常见 Windows 启动项及启用状态，不修改", false},
	{"chat.start", "新对话", "创建本盘维护对话", true},
	{"chat.list", "对话记录", "读取本盘维护对话", false},
	{"chat.read", "继续对话", "读取指定维护对话", false},
	{"chat.progress", "处理进度", "读取当前对话的实际动作结果", false},
	{"chat.send", "AI 维护对话", "使用公司 API 分析证据，调用受控维护动作", true},
	{"chat.confirm", "确认维护操作", "执行此对话中明确展示的操作", true},
	{"processes.inspect", "查看进程", "只读获取进程名称与内存，不读取命令行", false},
	{"network.rescue", "断网急救诊断", "分层检查网卡、网关、DNS、网页与代理，给出病因和下一步要打开的系统工具，不改配置", false},
	{"drivers.inspect", "网络设备与驱动", "离线读取网卡硬件 ID、驱动与故障码", false},
	{"tools.catalog", "维护工具集合", "查看Windows 系统工具的真实准备状态", false},
	{"tools.launch", "打开维护工具", "打开已注册的 Windows 系统工具，不自动清理或安装驱动", true},
	{"system.inspect", "电脑体检", "读取系统、内存与磁盘信息", false},
	{"network.inspect", "网络检查", "读取本机网络接口，不更改网络配置", false},
	{"report.create", "保存体检报告", "原子保存到本盘 data/reports，生成新的报告", true},
	{"reports.list", "查看维护记录", "读取本盘生成的报告", false},
}

func runAction(ctx context.Context, root, id string, input map[string]any) Result {
	var data any
	var err error
	switch id {
	case "settings.ui.get", "settings.ui.save":
		data, err = uiSettingsAction(root, id, input)
	case "settings.model.get", "settings.model.save", "settings.model.test":
		data, err = modelSettingsAction(ctx, root, id, input)
	case "startup.inspect":
		data, err = inspectStartup(ctx, root)
	case "chat.start", "chat.list", "chat.read", "chat.progress", "chat.send", "chat.confirm":
		data, err = chatAction(ctx, root, id, input)
	case "processes.inspect":
		data, err = inspectProcesses(ctx, root)
	case "network.rescue":
		data, err = networkRescue(ctx, root)
	case "drivers.inspect":
		data, err = inspectDrivers(ctx, root)
	case "tools.catalog":
		data, err = toolCatalog(root)
	case "tools.launch":
		data, err = launchTool(ctx, str(input, "tool_id"), input["confirmed"] == true)
	case "system.inspect":
		data, err = inspectSystem(root)
	case "network.inspect":
		data, err = inspectNetwork()
	case "report.create":
		data, err = createReport(root, input)
	case "reports.list":
		data, err = listReports(root)
	default:
		err = fmt.Errorf("不支持的动作：%s", id)
	}
	if err != nil {
		return Result{OK: false, Action: id, Error: err.Error()}
	}
	return Result{OK: true, Action: id, Data: data}
}

func str(input map[string]any, key string) string { s, _ := input[key].(string); return s }

func main() {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		fmt.Fprintln(os.Stderr, "网管精简版仅支持 Windows x64")
		os.Exit(1)
	}
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "version" || args[0] == "--version") {
		fmt.Println(version)
		return
	}
	if len(args) > 0 && args[0] == "action" {
		actionCLI(args[1:])
		return
	}
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}
	flags := flag.NewFlagSet("xiapan", flag.ExitOnError)
	rootFlag := flags.String("root", "", "便携目录，默认程序所在目录")
	port := flags.Int("port", 0, "本机端口，0为自动选择")
	noOpen := flags.Bool("no-open", false, "不自动打开浏览器")
	flags.Parse(args)
	root, err := resolveRoot(*rootFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	base := "http://" + listener.Addr().String()
	server, err := newServer(root, base)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "虾盘已启动："+base+" · 关闭此进程可停止服务")
	if !*noOpen {
		openBrowser(base)
	}
	if err = server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveRoot(root string) (string, error) {
	if root == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		root = filepath.Dir(exe)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("便携根路径必须是目录")
	}
	return filepath.EvalSymlinks(abs)
}

func actionCLI(args []string) {
	if len(args) == 0 || args[0] == "list" {
		json.NewEncoder(os.Stdout).Encode(Result{OK: true, Action: "action.list", Data: actions})
		return
	}
	if len(args) < 2 || args[0] != "run" {
		json.NewEncoder(os.Stdout).Encode(Result{OK: false, Error: "用法：action run <id> --json --input-file <JSON> --root <目录>"})
		os.Exit(2)
	}
	id := args[1]
	flags := flag.NewFlagSet("action run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rootFlag := flags.String("root", "", "便携根目录")
	jsonFlag := flags.Bool("json", false, "JSON 输出")
	_ = jsonFlag
	inputFile := flags.String("input-file", "", "JSON 参数文件")
	noInput := flags.Bool("no-input", false, "禁用交互")
	_ = noInput
	if err := flags.Parse(args[2:]); err != nil {
		json.NewEncoder(os.Stdout).Encode(Result{OK: false, Action: id, Error: err.Error()})
		os.Exit(2)
	}
	root, err := resolveRoot(*rootFlag)
	if err != nil {
		json.NewEncoder(os.Stdout).Encode(Result{OK: false, Action: id, Error: err.Error()})
		os.Exit(1)
	}
	input := map[string]any{}
	if *inputFile != "" {
		f, err := os.Open(*inputFile)
		if err != nil {
			json.NewEncoder(os.Stdout).Encode(Result{OK: false, Action: id, Error: err.Error()})
			os.Exit(1)
		}
		defer f.Close()
		if err = json.NewDecoder(io.LimitReader(f, 65537)).Decode(&input); err != nil {
			json.NewEncoder(os.Stdout).Encode(Result{OK: false, Action: id, Error: err.Error()})
			os.Exit(2)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r := runAction(ctx, root, id, input)
	json.NewEncoder(os.Stdout).Encode(r)
	if !r.OK {
		os.Exit(1)
	}

}

func newServer(root, base string) (*http.Server, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(key)
	parsed, _ := url.Parse(base)
	mux := http.NewServeMux()
	var server *http.Server
	mux.HandleFunc("/api/quit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		if r.Header.Get("X-Xiapan-Token") != token {
			http.Error(w, "需要本机虾盘会话", 403)
			return
		}
		writeJSON(w, Result{OK: true, Action: "app.quit"})
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			server.Shutdown(ctx)
		}()
	})
	mux.HandleFunc("/api/actions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		writeJSON(w, Result{OK: true, Data: actions})
	})
	mux.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", 405)
			return
		}
		if r.Header.Get("X-Xiapan-Token") != token {
			http.Error(w, "请从本机虾盘界面发起操作", 403)
			return
		}
		var req struct {
			Action string         `json:"action"`
			Input  map[string]any `json:"input"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&req); err != nil {
			writeJSON(w, Result{OK: false, Error: "请求格式不正确"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		writeJSON(w, runAction(ctx, root, req.Action, req.Input))
	})
	mux.HandleFunc("/api/skills", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		entries, err := fs.Glob(assets, "skills/*/SKILL.md")
		if err != nil {
			writeJSON(w, Result{OK: false, Error: err.Error()})
			return
		}
		var skills []map[string]string
		for _, p := range entries {
			b, _ := assets.ReadFile(p)
			skills = append(skills, map[string]string{"id": strings.Split(p, "/")[1], "content": string(b)})
		}
		writeJSON(w, Result{OK: true, Data: skills})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", 405)
			return
		}
		path := "web" + r.URL.Path
		if r.URL.Path == "/" {
			path = "web/index.html"
		}
		b, err := assets.ReadFile(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch filepath.Ext(path) {
		case ".html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			b = []byte(strings.ReplaceAll(string(b), "__XIAPAN_TOKEN__", token))
		case ".css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case ".js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		case ".json":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		w.Write(b)
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != parsed.Host {
			http.Error(w, "仅接受本机虾盘请求", 403)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != base {
			http.Error(w, "不接受跨站请求", 403)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		mux.ServeHTTP(w, r)
	})
	server = &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 30 * time.Second}
	return server, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func openBrowser(address string) {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", address)
	if err := cmd.Start(); err == nil {
		go cmd.Wait()
	}
}
