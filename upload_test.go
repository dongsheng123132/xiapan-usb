package main

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testUploadToken = "test-upload-token"

// uploadTestProject registers a fresh project directory under root and returns
// its id plus real absolute path.
func uploadTestProject(t *testing.T, root string) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "project")
	r := runAction(context.Background(), root, "workspace.create", map[string]any{"path": dir})
	if !r.OK {
		t.Fatalf("workspace.create: %v", r.Error)
	}
	got := r.Data.(map[string]any)
	id, _ := got["id"].(string)
	path, _ := got["path"].(string)
	if id == "" || path == "" {
		t.Fatalf("workspace.create returned %#v", got)
	}
	return id, path
}

// uploadTestRequest posts one multipart upload to the handler. The empty
// workspaceID/subdir omit their fields, matching what the web UI sends.
func uploadTestRequest(t *testing.T, root, token string, limit int64, workspaceID, subdir, field, fileName string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if workspaceID != "" {
		if err := writer.WriteField("workspace_id", workspaceID); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	if subdir != "" {
		if err := writer.WriteField("subdir", subdir); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}
	if field != "" {
		part, err := writer.CreateFormFile(field, fileName)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err = part.Write(content); err != nil {
			t.Fatalf("part.Write: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("X-Xiapan-Token", token)
	}
	recorder := httptest.NewRecorder()
	uploadHandlerWithLimit(root, testUploadToken, limit).ServeHTTP(recorder, request)
	return recorder
}

func decodeUploadError(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("error body is not JSON: %q", recorder.Body.String())
	}
	return payload["error"]
}

func TestUploadWritesFileIntoProject(t *testing.T) {
	root := t.TempDir()
	id, project := uploadTestProject(t, root)
	content := []byte("hello xiapan")

	recorder := uploadTestRequest(t, root, testUploadToken, 0, id, "", "file", "hello.txt", content)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d body %s", recorder.Code, recorder.Body.String())
	}
	var saved uploadResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &saved); err != nil {
		t.Fatalf("success body is not JSON: %q", recorder.Body.String())
	}
	if saved.Name != "hello.txt" {
		t.Fatalf("name %q want hello.txt", saved.Name)
	}
	if saved.Path != filepath.Join(project, "hello.txt") {
		t.Fatalf("path %q want %q", saved.Path, filepath.Join(project, "hello.txt"))
	}
	if saved.Size != int64(len(content)) {
		t.Fatalf("size %d want %d", saved.Size, len(content))
	}
	onDisk, err := os.ReadFile(saved.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(onDisk, content) {
		t.Fatalf("content %q want %q", onDisk, content)
	}
}

func TestUploadCreatesSingleLevelSubdir(t *testing.T) {
	root := t.TempDir()
	id, project := uploadTestProject(t, root)

	recorder := uploadTestRequest(t, root, testUploadToken, 0, id, "docs", "file", "note.md", []byte("note"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d body %s", recorder.Code, recorder.Body.String())
	}
	var saved uploadResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &saved); err != nil {
		t.Fatalf("success body is not JSON: %q", recorder.Body.String())
	}
	if saved.Path != filepath.Join(project, "docs", "note.md") {
		t.Fatalf("path %q want %q", saved.Path, filepath.Join(project, "docs", "note.md"))
	}
	if info, err := os.Stat(filepath.Join(project, "docs")); err != nil || !info.IsDir() {
		t.Fatalf("subdir was not created: %v", err)
	}
}

func TestUploadRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	id, project := uploadTestProject(t, root)
	parent := filepath.Dir(project)

	for _, subdir := range []string{"..", "../escape", "a/b", `/tmp`, `..\escape`, "."} {
		recorder := uploadTestRequest(t, root, testUploadToken, 0, id, subdir, "file", "evil.txt", []byte("x"))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("subdir %q: status %d body %s", subdir, recorder.Code, recorder.Body.String())
		}
		if message := decodeUploadError(t, recorder); !strings.Contains(message, "子目录") {
			t.Fatalf("subdir %q: error %q does not explain the subdirectory", subdir, message)
		}
	}
	// A traversal-style file name must be reduced to its base name inside the
	// project instead of escaping it.
	recorder := uploadTestRequest(t, root, testUploadToken, 0, id, "", "file", "../escape.txt", []byte("x"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d body %s", recorder.Code, recorder.Body.String())
	}
	var saved uploadResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &saved); err != nil {
		t.Fatalf("success body is not JSON: %q", recorder.Body.String())
	}
	if saved.Path != filepath.Join(project, "escape.txt") {
		t.Fatalf("path %q escaped the project root", saved.Path)
	}
	if matches, _ := filepath.Glob(filepath.Join(parent, "escape.txt")); len(matches) != 0 {
		t.Fatalf("file was written outside the project: %v", matches)
	}
}

func TestUploadNeverOverwritesExistingFile(t *testing.T) {
	root := t.TempDir()
	id, project := uploadTestProject(t, root)

	first := uploadTestRequest(t, root, testUploadToken, 0, id, "", "file", "same.txt", []byte("first"))
	if first.Code != http.StatusOK {
		t.Fatalf("first status %d body %s", first.Code, first.Body.String())
	}
	second := uploadTestRequest(t, root, testUploadToken, 0, id, "", "file", "same.txt", []byte("second"))
	if second.Code != http.StatusOK {
		t.Fatalf("second status %d body %s", second.Code, second.Body.String())
	}
	var saved uploadResponse
	if err := json.Unmarshal(second.Body.Bytes(), &saved); err != nil {
		t.Fatalf("success body is not JSON: %q", second.Body.String())
	}
	if saved.Name != "same-1.txt" {
		t.Fatalf("second name %q want same-1.txt", saved.Name)
	}
	original, err := os.ReadFile(filepath.Join(project, "same.txt"))
	if err != nil {
		t.Fatalf("ReadFile original: %v", err)
	}
	if string(original) != "first" {
		t.Fatalf("original file was overwritten: %q", original)
	}
	copied, err := os.ReadFile(filepath.Join(project, "same-1.txt"))
	if err != nil {
		t.Fatalf("ReadFile copy: %v", err)
	}
	if string(copied) != "second" {
		t.Fatalf("second file content %q want second", copied)
	}
}

func TestUploadRejectsOversizedFile(t *testing.T) {
	root := t.TempDir()
	id, project := uploadTestProject(t, root)

	recorder := uploadTestRequest(t, root, testUploadToken, 8, id, "", "file", "big.bin", bytes.Repeat([]byte("a"), 64))
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d want 413, body %s", recorder.Code, recorder.Body.String())
	}
	if message := decodeUploadError(t, recorder); !strings.Contains(message, "过大") {
		t.Fatalf("error %q does not mention the size limit", message)
	}
	entries, err := os.ReadDir(project)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected upload left files behind: %v", entries)
	}
}

func TestUploadRejectsInvalidWorkspace(t *testing.T) {
	root := t.TempDir()
	if _, err := os.Stat(filepath.Join(root, "data", "settings", "workspaces.json")); err == nil {
		t.Fatal("test root already has a workspace list")
	}
	for _, workspaceID := range []string{"missing", ""} {
		recorder := uploadTestRequest(t, root, testUploadToken, 0, workspaceID, "", "file", "x.txt", []byte("x"))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("workspace %q: status %d body %s", workspaceID, recorder.Code, recorder.Body.String())
		}
		message := decodeUploadError(t, recorder)
		if message == "" {
			t.Fatalf("workspace %q: empty error message", workspaceID)
		}
	}

	// A registered project whose directory disappeared must report that, not
	// guess a replacement.
	id, project := uploadTestProject(t, root)
	if err := os.RemoveAll(project); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	recorder := uploadTestRequest(t, root, testUploadToken, 0, id, "", "file", "x.txt", []byte("x"))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", recorder.Code, recorder.Body.String())
	}
	if message := decodeUploadError(t, recorder); !strings.Contains(message, "本机不存在") {
		t.Fatalf("error %q does not explain the missing project", message)
	}
}

func TestUploadRequiresLocalToken(t *testing.T) {
	root := t.TempDir()
	id, _ := uploadTestProject(t, root)
	for _, token := range []string{"", "wrong"} {
		recorder := uploadTestRequest(t, root, token, 0, id, "", "file", "x.txt", []byte("x"))
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("token %q: status %d body %s", token, recorder.Code, recorder.Body.String())
		}
	}
}

func TestUploadRejectsMissingFilePart(t *testing.T) {
	root := t.TempDir()
	id, _ := uploadTestProject(t, root)
	recorder := uploadTestRequest(t, root, testUploadToken, 0, id, "", "", "", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", recorder.Code, recorder.Body.String())
	}
	if message := decodeUploadError(t, recorder); !strings.Contains(message, "没有收到文件") {
		t.Fatalf("error %q does not explain the missing file", message)
	}
}

func TestSanitizeUploadName(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"report.pdf", "report.pdf"},
		{"../../evil.txt", "evil.txt"},
		{`..\..\evil.txt`, "evil.txt"},
		{"a/b/c.txt", "c.txt"},
		{`bad:na?me*.txt`, "badname.txt"},
		{"  报告.docx  ", "报告.docx"},
	}
	for _, testCase := range cases {
		got, err := sanitizeUploadName(testCase.raw)
		if err != nil {
			t.Fatalf("sanitizeUploadName(%q): %v", testCase.raw, err)
		}
		if got != testCase.want {
			t.Fatalf("sanitizeUploadName(%q) = %q want %q", testCase.raw, got, testCase.want)
		}
	}
	for _, raw := range []string{"", "   ", ".", ".."} {
		if got, err := sanitizeUploadName(raw); err == nil {
			t.Fatalf("sanitizeUploadName(%q) = %q want an error", raw, got)
		}
	}
}
