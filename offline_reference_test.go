package main

import (
	"context"
	"strings"
	"testing"
)

func TestReferencePrecedesInspectionAndInference(t *testing.T) {
	for _, question := range []string{
		"硬盘咔哒异响，文件不见了，应该怎么办？",
		"只有 Codex 提示无法加载组织设置，其他网站正常。",
		"插上网线仍不能上网，怎么排查？",
		"误删文件后要如何处理？",
	} {
		root := t.TempDir()
		markOfflineTestRoot(t, root)
		v, err := chatAction(context.Background(), root, "chat.start", map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		s := v.(ChatSession)
		v, err = chatAction(context.Background(), root, "chat.send", map[string]any{"session_id": s.ID, "expected_version": float64(s.Version), "engine": "local", "message": question})
		if err != nil {
			t.Fatal(err)
		}
		s = v.(ChatSession)
		if len(s.Messages) != 2 || s.Messages[1].Role != "assistant" || !strings.HasPrefix(s.Messages[1].Text, "维护知识指引（未调用模型）") {
			t.Fatalf("reference must not perform inspections or call a missing model: %#v", s.Messages)
		}
		if len(s.Pending) != 0 {
			t.Fatal("reference proposed an action")
		}
	}
}

func TestPhysicalDiskFollowupRetainsReferenceAndOrdinaryQuestionsUseModel(t *testing.T) {
	s := &ChatSession{OfflineOnly: true, Messages: []ChatMessage{{Role: "user", Text: "我的硬盘有咔哒异响"}, {Role: "assistant", Text: "指引"}, {Role: "user", Text: "那我可以直接扫描吗？"}}}
	if reply := offlineReferenceReply(s, "那我可以直接扫描吗？", "zh-CN"); !strings.Contains(reply, "不要先运行 PhotoRec/TestDisk") {
		t.Fatal(reply)
	}
	if reply := offlineReferenceReply(&ChatSession{OfflineOnly: true}, "请解释 SMART 健康信息的含义和局限。", "zh-CN"); reply != "" {
		t.Fatal("ordinary model question was intercepted")
	}
	if reply := offlineReferenceReply(&ChatSession{}, "硬盘异响", "zh-CN"); reply != "" {
		t.Fatal("ordinary edition changed")
	}
}
