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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type CloudConfig struct {
	API       string `json:"api_base_url"`
	Pay       string `json:"pay_base_url"`
	Preferred string `json:"preferred_model"`
}

func cloudConfig() CloudConfig {
	var c CloudConfig
	b, _ := assets.ReadFile("catalog/cloud.json")
	json.Unmarshal(b, &c)
	if v := os.Getenv("UCLAW_API_BASE_URL"); v != "" {
		c.API = strings.TrimRight(v, "/")
	}
	if v := os.Getenv("UCLAW_PAY_BASE_URL"); v != "" {
		c.Pay = strings.TrimRight(v, "/")
	}
	return c
}

// Only operator-supplied configuration may select an endpoint, never model text.
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
			return errors.New("凭证未通过验证，请检查所选服务的 API Key 或钱包")
		case 402, 403:
			return errors.New("云端拒绝本次请求；可能是额度不足或模型权限受限，请检查钱包")
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

type WalletState struct {
	APIKey      string `json:"apiKey,omitempty"`
	WalletID    string `json:"walletId,omitempty"`
	PendingKey  string `json:"pendingKey,omitempty"`
	PendingKind string `json:"pendingKind,omitempty"`
	PendingFrom string `json:"pendingFrom,omitempty"`
}
type WalletView struct {
	Ready     bool     `json:"ready"`
	WalletID  string   `json:"wallet_id,omitempty"`
	MaskedKey string   `json:"masked_key,omitempty"`
	Available *float64 `json:"available_tokens,omitempty"`
	Used      *float64 `json:"used_tokens,omitempty"`
	Note      string   `json:"note"`
	Pending   bool     `json:"pending"`
}

var walletLocks sync.Map

// Optional engines register once; the wallet and chat remain usable when an
// adapter module is omitted from a build.
var nativeAgent struct {
	ApplyKey func(string, string) error
	Chat     func(context.Context, string, *ChatSession, string, string) (string, error)
}
var errWalletCorrupt = errors.New("钱包文件损坏，已保留原文件；可填入备份的凭证")

func rootLock(m *sync.Map, root string) *sync.Mutex {
	v, _ := m.LoadOrStore(root, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func loadWallet(root string) (WalletState, error) {
	p, err := writablePath(root, "data", "wallet", "device.json")
	if err != nil {
		return WalletState{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return WalletState{}, nil
	}
	if err != nil {
		return WalletState{}, errors.New("无法读取本盘钱包；工具仍可使用")
	}
	var s WalletState
	// Preserve a damaged wallet instead of replacing it with a zero-balance one.
	// The UI always offers adopt, so a backed-up credential can recover access.
	if len(b) > 65536 || json.Unmarshal(b, &s) != nil {
		return WalletState{}, errWalletCorrupt
	}
	return s, nil
}

func replaceLocalFile(root string, parts []string, b []byte) error {
	p, err := writablePath(root, parts...)
	if err != nil {
		return err
	}
	old, readErr := os.ReadFile(p)
	if readErr == nil {
		if bytes.Equal(old, b) {
			return nil
		}
		if _, err = newFileAtomic(root, []string{"data", "backups", uniqueID() + "-" + filepath.Base(p)}, old); err != nil {
			return err
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".xiapan-save-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), p)
}
func saveWallet(root string, s WalletState) error {
	b, _ := json.MarshalIndent(s, "", "  ")
	return replaceLocalFile(root, []string{"data", "wallet", "device.json"}, b)
}

// Every consumer reads this one state. No browser/session/host copy of the key.
func walletKey(root string) (string, error) {
	lock := rootLock(&walletLocks, root)
	lock.Lock()
	defer lock.Unlock()
	s, err := loadWallet(root)
	if err != nil {
		return "", err
	}
	if s.APIKey == "" {
		return "", errors.New("联网获取钱包，或填入已有钱包后再使用云端 AI")
	}
	return s.APIKey, nil
}
func applyWalletKey(root, key string) error {
	// The HTTP consumer reads walletKey per request. The optional PicoClaw
	// adapter materializes its credential here, never in a second code path.
	if nativeAgent.ApplyKey != nil {
		return nativeAgent.ApplyKey(root, key)
	}
	return nil
}
func maskedKey(key string) string {
	if len(key) < 8 {
		return "已保存凭证"
	}
	return "••••••••" + key[len(key)-4:]
}
func walletView(s WalletState, note string) WalletView {
	v := WalletView{Ready: s.APIKey != "", WalletID: s.WalletID, Note: note, Pending: s.PendingKey != ""}
	if v.Ready {
		v.MaskedKey = maskedKey(s.APIKey)
	}
	return v
}

func validateWallet(ctx context.Context, key string) (map[string]any, error) {
	var data map[string]any
	err := remoteJSON(ctx, cloudConfig().API, "/api/usage/token/", "GET", key, nil, &data)
	if err != nil {
		return nil, err
	}
	if ok, exists := data["success"].(bool); exists && !ok {
		return nil, errors.New("钱包未通过只读验证")
	}
	if inner, ok := data["data"].(map[string]any); ok {
		data = inner
	}
	if _, ok := data["total_available"].(float64); !ok {
		return nil, errors.New("钱包响应缺少余额；未将查询失败当作零余额")
	}
	return data, nil
}
func queryWallet(ctx context.Context, s WalletState, note string) WalletView {
	v := walletView(s, note)
	if !v.Ready {
		return v
	}
	cancelCtx, cancel := context.WithTimeout(ctx, 9*time.Second)
	defer cancel()
	d, err := validateWallet(cancelCtx, s.APIKey)
	if err != nil {
		v.Note = err.Error()
		return v
	}
	available := d["total_available"].(float64)
	v.Available = &available
	if used, ok := d["total_used"].(float64); ok {
		v.Used = &used
	}
	return v
}
func settleWallet(ctx context.Context, root string, s WalletState) (WalletState, error) {
	if s.PendingKey == "" {
		return s, nil
	}
	if s.PendingKind != "rotate" || s.PendingFrom == "" {
		return s, errors.New("有无法识别的未完成钱包操作，已保留状态；不会新建或覆盖")
	}
	if _, err := validateWallet(ctx, s.PendingKey); err != nil {
		return s, err
	}
	var reply map[string]any
	if err := remoteJSON(ctx, cloudConfig().API, "/device/rotate/commit", "POST", "", map[string]string{"currentKey": s.PendingFrom, "newKey": s.PendingKey}, &reply); err != nil {
		return s, err
	}
	s.APIKey = s.PendingKey
	s.PendingKey = ""
	s.PendingKind = ""
	s.PendingFrom = ""
	if err := saveWallet(root, s); err != nil {
		return s, err
	}
	return s, applyWalletKey(root, s.APIKey)
}
func ensureWallet(ctx context.Context, root string) WalletView {
	lock := rootLock(&walletLocks, root)
	lock.Lock()
	defer lock.Unlock()
	s, err := loadWallet(root)
	if err != nil {
		return walletView(s, err.Error())
	}
	if s.PendingKey != "" {
		s, err = settleWallet(ctx, root, s)
		if err != nil {
			return walletView(s, err.Error())
		}
	}
	if s.APIKey != "" {
		if err := applyWalletKey(root, s.APIKey); err != nil {
			return walletView(s, "钱包保留，但原生引擎配置未同步")
		}
		return queryWallet(ctx, s, "凭证就是钱包，请自行备份；无需注册")
	}
	var issue struct {
		Key string `json:"apiKey"`
		ID  string `json:"walletId"`
	}
	requestCtx, cancel := context.WithTimeout(ctx, 9*time.Second)
	defer cancel()
	err = remoteJSON(requestCtx, cloudConfig().API, "/device/bind", "POST", "", map[string]string{"hwHint": "", "platform": runtime.GOOS, "channel": "xiapan-maintenance-usb"}, &issue)
	if err != nil {
		return walletView(s, "联网后自动获取设备钱包；本地维护工具可继续使用")
	}
	if issue.Key == "" || issue.ID == "" {
		return walletView(s, "钱包服务响应不完整，未保存")
	}
	s = WalletState{APIKey: issue.Key, WalletID: issue.ID}
	if err = saveWallet(root, s); err != nil {
		return walletView(WalletState{}, "U 盘不可写，未完成钱包保存；工具仍可使用")
	}
	if err = applyWalletKey(root, s.APIKey); err != nil {
		return walletView(s, "钱包已保存，原生引擎配置暂未同步")
	}
	return queryWallet(ctx, s, "新钱包已创建；余额以实时查询为准，可充值或填入已有钱包")
}
func walletAction(ctx context.Context, root, id string, input map[string]any) (any, error) {
	if id == "wallet.ensure" {
		return ensureWallet(ctx, root), nil
	}
	lock := rootLock(&walletLocks, root)
	lock.Lock()
	defer lock.Unlock()
	s, err := loadWallet(root)
	if err != nil && !(id == "wallet.adopt" && errors.Is(err, errWalletCorrupt)) {
		return nil, err
	}
	switch id {
	case "wallet.refresh":
		return queryWallet(ctx, s, "凭证就是钱包，请自行备份"), nil
	case "wallet.copy":
		if input["confirmed"] != true || s.APIKey == "" {
			return nil, errors.New("请主动选择备份钱包凭证")
		}
		return map[string]string{"api_key": s.APIKey}, nil
	case "wallet.adopt":
		if s.PendingKey != "" {
			return nil, errors.New("请先完成未结束的钱包操作")
		}
		key := strings.TrimSpace(str(input, "api_key"))
		if !strings.HasPrefix(key, "sk-") || len(key) < 12 || len(key) > 256 || strings.ContainsAny(key, "\r\n\t ") {
			return nil, errors.New("请输入有效的钱包凭证")
		}
		if _, err = validateWallet(ctx, key); err != nil {
			return nil, err
		}
		s = WalletState{APIKey: key}
		if err = saveWallet(root, s); err != nil {
			return nil, err
		}
		if err = applyWalletKey(root, key); err != nil {
			return nil, err
		}
		return queryWallet(ctx, s, "已使用已有钱包；余额没有搬动"), nil
	case "wallet.rotate":
		if input["confirmed"] != true {
			return nil, errors.New("换一把会使旧凭证失效，请明确确认")
		}
		if s.APIKey == "" {
			return nil, errors.New("当前没有钱包")
		}
		if s.PendingKey == "" {
			var issue struct {
				Key string `json:"apiKey"`
				ID  string `json:"walletId"`
			}
			if err = remoteJSON(ctx, cloudConfig().API, "/device/rotate", "POST", "", map[string]string{"currentKey": s.APIKey}, &issue); err != nil {
				return nil, err
			}
			if issue.Key == "" {
				return nil, errors.New("轮换响应不完整")
			}
			s.PendingKey = issue.Key
			s.PendingKind = "rotate"
			s.PendingFrom = s.APIKey
			if err = saveWallet(root, s); err != nil {
				return nil, err
			}
		}
		s, err = settleWallet(ctx, root, s)
		if err != nil {
			return nil, err
		}
		return queryWallet(ctx, s, "已换一把；余额仍在同一钱包，旧凭证已失效"), nil
	case "wallet.reset":
		if input["confirmed"] != true || input["backed_up"] != true {
			return nil, errors.New("先备份凭证，再确认移除；旧余额不会进入新钱包")
		}
		if s.PendingKey != "" {
			return nil, errors.New("有未完成的钱包操作，不能移除")
		}
		if err = applyWalletKey(root, ""); err != nil {
			return nil, err
		}
		if err = saveWallet(root, WalletState{}); err != nil {
			return nil, err
		}
		return walletView(WalletState{}, "本盘钱包已移除；服务端钱包、凭证和余额保留"), nil
	case "wallet.recharge":
		if s.APIKey == "" {
			return nil, errors.New("请联网获取钱包或填入已有钱包")
		}
		address, err := serviceURL(cloudConfig().Pay, "/recharge")
		if err != nil {
			return nil, err
		}
		u, _ := url.Parse(address)
		q := u.Query()
		q.Set("key", s.APIKey)
		u.RawQuery = q.Encode()
		if input["check_only"] == true {
			req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
			resp, err := (&http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
			if err != nil {
				return nil, errors.New("充值页面暂时无法连接")
			}
			defer resp.Body.Close()
			return map[string]any{"status": resp.StatusCode, "payment_made": false}, nil
		}
		openBrowser(u.String())
		return map[string]any{"opened": true, "payment_made": false}, nil
	}
	return nil, errors.New("未知钱包动作")
}

func cloudModels(ctx context.Context, root string) (any, error) {
	key, _ := walletKey(root)
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := remoteJSON(ctx, cloudConfig().API, "/v1/models", "GET", key, nil, &out); err != nil {
		return nil, err
	}
	models := []string{}
	for _, m := range out.Data {
		if m.ID != "" {
			models = append(models, m.ID)
		}
	}
	// The public price metadata distinguishes token-billed models from
	// per-request media products. Do not maintain a second model-name list.
	var pricing struct {
		Data []struct {
			Name      string `json:"model_name"`
			QuotaType *int   `json:"quota_type"`
		} `json:"data"`
	}
	priceCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	priceErr := remoteJSON(priceCtx, cloudConfig().API, "/api/pricing", "GET", "", nil, &pricing)
	cancel()
	filtered := false
	if priceErr == nil && len(pricing.Data) > 0 {
		allowed := map[string]bool{}
		for _, p := range pricing.Data {
			if p.QuotaType != nil && *p.QuotaType == 0 {
				allowed[p.Name] = true
			}
		}
		textModels := []string{}
		for _, m := range models {
			if allowed[m] {
				textModels = append(textModels, m)
			}
		}
		if len(textModels) > 0 {
			models = textModels
			filtered = true
		}
	}
	chosen := ""
	preferred := cloudConfig().Preferred
	for _, m := range models {
		if m == preferred {
			chosen = m
			break
		}
	}
	if chosen == "" && len(models) > 0 {
		chosen = models[0]
	}
	return map[string]any{"models": models, "selected": chosen, "source": "service", "filtered_by_token_billing": filtered}, nil
}
