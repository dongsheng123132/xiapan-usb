//go:build !darwin

package main

import "errors"

func hostDetails(info *SystemInfo) {}

func translocatedOrigin(path string) (string, bool) { return "", false }

func launchBundle(exe string) error { return errors.New("仅 macOS 应用包使用此启动方式") }

func alertUser(title, message string) {}
