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
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type AIManifest struct {
	Ready       bool   `json:"ready"`
	Platform    string `json:"platform"`
	ModelName   string `json:"model_name"`
	ModelPath   string `json:"model_path"`
	ModelSHA    string `json:"model_sha256"`
	EnginePath  string `json:"engine_path"`
	EngineFiles []struct {
		Path string `json:"path"`
		SHA  string `json:"sha256"`
	} `json:"engine_files"`
}

var modelMu sync.Mutex
var modelKeys sync.Map
var modelAddresses sync.Map

type modelProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	log  *os.File
}

var modelProcesses = map[string]*modelProcess{}

func aiManifest() (AIManifest, error) {
	var m AIManifest
	b, err := assets.ReadFile("catalog/local-ai.json")
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

// Compact packages keep program resources under app/. Existing USB layouts
// remain supported. Both paths must pass the same containment checks.
func runtimeResourcePath(root, rel string) (string, error) {
	path, err := writablePath(root, strings.Split(rel, "/")...)
	if err != nil {
		return "", err
	}
	if _, err = os.Lstat(path); errors.Is(err, os.ErrNotExist) && (rel == "runtime" || strings.HasPrefix(rel, "runtime/")) {
		return writablePath(root, strings.Split("app/"+rel, "/")...)
	}
	return path, nil
}

func verifyResource(root, rel, expected string) (string, error) {
	path, err := runtimeResourcePath(root, rel)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("缺少本地资源：%s", rel)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("本地资源不是普通文件")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	if hex.EncodeToString(h.Sum(nil)) != expected {
		return "", fmt.Errorf("本地资源校验不通过：%s", rel)
	}
	return path, nil
}

func bundledAI(root string) (bool, string) {
	m, err := aiManifest()
	if err != nil || !m.Ready {
		return false, "此构建没有准备可校验的模型运行包"
	}
	if m.Platform != runtime.GOOS+"/"+runtime.GOARCH {
		return false, "模型运行包与当前系统不匹配"
	}
	for _, rel := range []string{m.ModelPath, m.EnginePath} {
		path, err := writablePath(root, strings.Split(rel, "/")...)
		if err != nil {
			return false, err.Error()
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return false, "尚未备齐本地模型资源"
		}
	}
	return true, "本盘模型与运行时已备齐，启动前会校验文件"
}

func startModel(ctx context.Context, root string, input map[string]any) (any, error) {
	if input["confirmed"] != true {
		return nil, errors.New("请确认启动本盘模型；将使用电脑的 CPU 和内存")
	}
	m, err := aiManifest()
	if err != nil {
		return nil, err
	}
	if !m.Ready || m.Platform != runtime.GOOS+"/"+runtime.GOARCH {
		return nil, errors.New("当前构建没有匹配此系统的本地推理资源")
	}
	modelMu.Lock()
	defer modelMu.Unlock()
	if existing := modelProcesses[root]; existing != nil {
		select {
		case <-existing.done:
			delete(modelProcesses, root)
		default:
			return map[string]any{"ready": true, "model": m.ModelName, "already_running": true}, nil
		}
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	address := reservation.Addr().String()
	port := reservation.Addr().(*net.TCPAddr).Port
	reservation.Close()
	modelPath, err := verifyResource(root, m.ModelPath, m.ModelSHA)
	if err != nil {
		return nil, err
	}
	for _, f := range m.EngineFiles {
		if _, err = verifyResource(root, f.Path, f.SHA); err != nil {
			return nil, err
		}
	}
	enginePath, err := writablePath(root, strings.Split(m.EnginePath, "/")...)
	if err != nil {
		return nil, err
	}
	logPath, err := writablePath(root, "data", "logs", "model-"+uniqueID()+".log")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	keyBytes := make([]byte, 32)
	if _, err = rand.Read(keyBytes); err != nil {
		logFile.Close()
		return nil, err
	}
	key := hex.EncodeToString(keyBytes)
	threads := runtime.NumCPU() / 2
	if threads > 8 {
		threads = 8
	}
	if threads < 1 {
		threads = 1
	}
	cmd := exec.Command(enginePath, "-m", modelPath, "--alias", m.ModelName, "--host", "127.0.0.1", "--port", fmt.Sprint(port), "-c", "4096", "-t", fmt.Sprint(threads), "-ngl", "0", "--jinja", "--reasoning-budget", "0", "--cors-origins", "localhost")
	cmd.Dir = filepath.Dir(enginePath)
	cmd.Env, err = portableEnv(root)
	if err != nil {
		logFile.Close()
		return nil, err
	}
	// The upstream API-key file loader does not accept Chinese paths in this build.
	// Keep this temporary local-service secret in memory and in the child environment.
	// It is never a command-line argument, disk credential or device-wallet key.
	cmd.Env = append(cmd.Env, "LLAMA_API_KEY="+key)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	quietProcess(cmd)
	if err = cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("本地模型启动失败：%w", err)
	}
	p := &modelProcess{cmd: cmd, done: make(chan struct{}), log: logFile}
	modelProcesses[root] = p
	modelKeys.Store("bundled", key)
	modelAddresses.Store("bundled", "http://"+address)
	go func() {
		cmd.Wait()
		logFile.Close()
		modelKeys.CompareAndDelete("bundled", key)
		modelAddresses.CompareAndDelete("bundled", "http://"+address)
		close(p.done)
	}()
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			return nil, errors.New("启动超时，本次启动的模型进程已停止")
		case <-p.done:
			return nil, errors.New("推理程序启动后退出，请查看本盘 data/logs 中的模型日志")
		case <-ticker.C:
			rows, err := queryLocalModels(ctx, "bundled")
			if err == nil {
				for _, s := range rows {
					if s.Model == m.ModelName {
						return map[string]any{"ready": true, "model": m.ModelName, "pid": cmd.Process.Pid, "port": port, "backend": "bundled", "mode": "CPU-only", "cloud_requests": 0}, nil
					}
				}
			}
		}
	}
}

func stopModel(root string) (any, error) {
	modelMu.Lock()
	defer modelMu.Unlock()
	p := modelProcesses[root]
	if p == nil {
		return map[string]any{"stopped": false, "note": "本程序没有在此目录启动模型，不会停止其他本地服务"}, nil
	}
	select {
	case <-p.done:
	default:
		p.cmd.Process.Kill()
	}
	select {
	case <-p.done:
	case <-time.After(3 * time.Second):
		return nil, errors.New("模型正在退出，请稍后重试")
	}
	delete(modelProcesses, root)
	return map[string]any{"stopped": true}, nil
}
func stopAllModels() {
	modelMu.Lock()
	defer modelMu.Unlock()
	for root, p := range modelProcesses {
		select {
		case <-p.done:
		default:
			p.cmd.Process.Kill()
		}
		select {
		case <-p.done:
		case <-time.After(3 * time.Second):
		}
		delete(modelProcesses, root)
	}
}
