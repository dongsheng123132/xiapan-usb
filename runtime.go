package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

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
