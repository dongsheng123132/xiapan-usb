package main

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
)

// UI preferences travel with the drive, independently of model credentials.
type UISettings struct {
	Revision string `json:"revision"`
	Language string `json:"language"`
}

var uiSettingsLocks sync.Map

func loadUISettings(root string) (UISettings, error) {
	s := UISettings{Revision: "initial", Language: "auto"}
	p, err := writablePath(root, "data", "settings", "ui.json")
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, errors.New("无法读取界面语言设置")
	}
	if len(b) > 4096 || json.Unmarshal(b, &s) != nil || !validLanguage(s.Language) || s.Revision == "" {
		return s, errors.New("界面语言设置损坏，原文件已保留")
	}
	return s, nil
}

func validLanguage(language string) bool {
	return language == "auto" || language == "zh-CN" || language == "en"
}

func uiSettingsAction(root, id string, input map[string]any) (any, error) {
	lock := rootLock(&uiSettingsLocks, root)
	lock.Lock()
	defer lock.Unlock()
	s, err := loadUISettings(root)
	if err != nil {
		return nil, err
	}
	if id == "settings.ui.get" {
		return s, nil
	}
	if id != "settings.ui.save" {
		return nil, errors.New("未知界面设置动作")
	}
	if str(input, "expected_revision") != s.Revision {
		return nil, errors.New("界面语言设置已变化，请刷新后重试")
	}
	language := str(input, "language")
	if !validLanguage(language) {
		return nil, errors.New("仅支持简体中文、English 或自动选择")
	}
	if language == s.Language {
		return s, nil
	}
	s.Language, s.Revision = language, uniqueID()
	b, _ := json.MarshalIndent(s, "", "  ")
	if err = replaceLocalFile(root, []string{"data", "settings", "ui.json"}, b); err != nil {
		return nil, err
	}
	return s, nil
}

func requestLanguage(root string, input map[string]any) string {
	if language := str(input, "locale"); language == "en" || language == "zh-CN" {
		return language
	}
	lock := rootLock(&uiSettingsLocks, root)
	lock.Lock()
	defer lock.Unlock()
	if s, err := loadUISettings(root); err == nil && s.Language == "en" {
		return "en"
	}
	return "zh-CN"
}

func responseLanguage(language string) string {
	if language == "en" {
		return "Respond in concise English unless the user explicitly requests another language."
	}
	return "用简洁中文回答，除非用户明确要求其他语言。"
}
