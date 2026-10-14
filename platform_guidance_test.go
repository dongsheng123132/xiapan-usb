package main

import (
	"strings"
	"testing"
)

func TestPlatformGuidance(t *testing.T) {
	for _, tc := range []struct{ platform, question, action, required, forbidden string }{
		{"darwin", "断网了怎么办", "drivers.inspect", "无线诊断", "硬件 ID"},
		{"darwin", "电脑无法启动", "tools.catalog", "macOS 恢复", "救援镜像"},
		{"windows", "断网了怎么办", "drivers.inspect", "驱动", "macOS"},
		{"windows", "电脑无法启动", "resources.scan", "救援", "Command-R"},
	} {
		t.Run(tc.platform+tc.question, func(t *testing.T) {
			p := guideForPlatform(tc.question, tc.platform)
			text := p.Summary + strings.Join(p.Steps, " ")
			if p.Action != tc.action || !strings.Contains(text, tc.required) || strings.Contains(text, tc.forbidden) {
				t.Fatalf("wrong platform guidance: %+v", p)
			}
		})
	}
}
