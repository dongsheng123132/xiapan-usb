package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// maxUploadBytes limits one uploaded file. The browser sends a single file per
// request; a bigger body is rejected before anything is written to the project.
const maxUploadBytes int64 = 200 << 20 // 200 MiB

// uploadResponse is the documented success shape of POST /api/upload:
// {"name":"report.pdf","path":"/Users/me/work/report.pdf","size":12345}
type uploadResponse struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// errUploadTooLarge marks a body that exceeded the configured per-file limit so
// the handler can answer 413 instead of a generic 400.
var errUploadTooLarge = errors.New("上传文件过大")

// uploadHandler serves POST /api/upload. It reuses the same local-session token
// as every other /api endpoint and never performs a network call.
func uploadHandler(root, token string) http.HandlerFunc {
	return uploadHandlerWithLimit(root, token, maxUploadBytes)
}

// uploadHandlerWithLimit is the test seam for the size limit. The multipart body
// is streamed once: metadata fields are read first (the web UI always sends
// workspace_id before the file), then the file part is written to a temporary
// file inside the destination project and renamed into place.
func uploadHandlerWithLimit(root, token string, limit int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("X-Xiapan-Token") != token {
			http.Error(w, "请从本机虾盘界面发起上传", http.StatusForbidden)
			return
		}
		if limit <= 0 {
			limit = maxUploadBytes
		}
		reader, err := r.MultipartReader()
		if err != nil {
			uploadError(w, http.StatusBadRequest, "上传请求格式不正确")
			return
		}
		var workspaceID, subdir string
		for {
			part, nextErr := reader.NextPart()
			if nextErr == io.EOF {
				break
			}
			if nextErr != nil {
				uploadError(w, http.StatusBadRequest, "上传请求读取中断，请重试")
				return
			}
			if part.FileName() == "" {
				value, readErr := io.ReadAll(io.LimitReader(part, 4096))
				part.Close()
				if readErr != nil {
					uploadError(w, http.StatusBadRequest, "上传请求读取中断，请重试")
					return
				}
				switch part.FormName() {
				case "workspace_id":
					workspaceID = strings.TrimSpace(string(value))
				case "subdir":
					subdir = strings.TrimSpace(string(value))
				}
				continue
			}
			// The file part: resolve the project before writing anything, so a
			// rejected request leaves no partial file behind.
			dir, dirErr := uploadTargetDir(root, workspaceID, subdir)
			if dirErr != nil {
				part.Close()
				uploadError(w, http.StatusBadRequest, dirErr.Error())
				return
			}
			name, nameErr := sanitizeUploadName(part.FileName())
			if nameErr != nil {
				part.Close()
				uploadError(w, http.StatusBadRequest, nameErr.Error())
				return
			}
			saved, saveErr := writeUploadPart(dir, name, part, limit)
			part.Close()
			if saveErr != nil {
				status := http.StatusBadRequest
				if errors.Is(saveErr, errUploadTooLarge) {
					status = http.StatusRequestEntityTooLarge
				}
				uploadError(w, status, saveErr.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(saved)
			return
		}
		uploadError(w, http.StatusBadRequest, "上传未完成：没有收到文件")
	}
}

// uploadTargetDir resolves the registered project's real absolute directory and
// an optional single-level subdirectory inside it. Anything that could escape
// the project (separators, "..", absolute paths, symlinks) is rejected.
func uploadTargetDir(root, workspaceID, subdir string) (string, error) {
	resolvedID := strings.TrimSpace(workspaceID)
	if resolvedID == "" {
		return "", errors.New("请先选择项目")
	}
	project, err := workspaceProjectRealPath(root, resolvedID)
	if err != nil {
		return "", err
	}
	subdir = strings.TrimSpace(subdir)
	if subdir == "" {
		return project, nil
	}
	if subdir == "." || subdir == ".." ||
		strings.ContainsAny(subdir, `/\:`) ||
		strings.ContainsRune(subdir, '\x00') ||
		filepath.Base(subdir) != subdir {
		return "", errors.New("子目录无效，只支持项目内的单层子目录")
	}
	target := filepath.Join(project, subdir)
	rel, err := filepath.Rel(project, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("子目录无效，只支持项目内的单层子目录")
	}
	info, statErr := os.Lstat(target)
	switch {
	case statErr == nil:
		// Reuse an existing real directory only; a symlink would let the upload
		// land outside the project.
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", errors.New("子目录无效，只支持项目内的单层子目录")
		}
	case errors.Is(statErr, os.ErrNotExist):
		if mkErr := os.Mkdir(target, 0700); mkErr != nil {
			return "", errors.New("无法创建子目录，请检查磁盘权限")
		}
	default:
		return "", errors.New("无法读取子目录，请检查磁盘权限")
	}
	return target, nil
}

// sanitizeUploadName keeps only a base name and removes characters that are not
// valid in a portable file name. The caller still has to reject empty results.
func sanitizeUploadName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	// Browsers send a bare name already; normalise separators defensively in
	// case another client sends a path.
	name = strings.ReplaceAll(name, `\`, "/")
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', 0:
			return -1
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	// Windows silently drops trailing dots/spaces; do it explicitly so the name
	// we report is the name on disk.
	name = strings.TrimRight(name, " .")
	if name == "" {
		return "", errors.New("文件名不能为空")
	}
	if name == "." || name == ".." {
		return "", errors.New("文件名无效，请重命名后再上传")
	}
	if len(name) > 180 {
		return "", errors.New("文件名过长，请重命名后再上传")
	}
	return name, nil
}

// writeUploadPart streams one multipart file part to a temporary file in dir and
// then renames it to a free name, so an existing file is never replaced.
func writeUploadPart(dir, name string, src io.Reader, limit int64) (uploadResponse, error) {
	tmp, err := os.CreateTemp(dir, ".xiapan-upload-*")
	if err != nil {
		return uploadResponse{}, errors.New("无法写入项目目录，请检查磁盘权限")
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		tmp.Close()
		if !committed {
			os.Remove(tmpName)
		}
	}()
	written, err := io.Copy(tmp, io.LimitReader(src, limit+1))
	if err != nil {
		return uploadResponse{}, errors.New("上传中断，文件未保存")
	}
	if written > limit {
		return uploadResponse{}, fmt.Errorf("%w，上限 %s", errUploadTooLarge, uploadLimitText(limit))
	}
	if err = tmp.Sync(); err != nil {
		return uploadResponse{}, errors.New("无法写入项目目录，请检查磁盘权限")
	}
	if err = tmp.Close(); err != nil {
		return uploadResponse{}, errors.New("无法写入项目目录，请检查磁盘权限")
	}
	final, err := reserveUploadName(dir, name)
	if err != nil {
		return uploadResponse{}, err
	}
	if err = os.Rename(tmpName, final); err != nil {
		// Remove the empty reservation; the content temp file is removed by the
		// deferred cleanup.
		os.Remove(final)
		return uploadResponse{}, errors.New("无法保存文件，请检查磁盘权限")
	}
	committed = true
	return uploadResponse{Name: filepath.Base(final), Path: final, Size: written}, nil
}

// reserveUploadName creates an empty file under a name that is still free,
// appending -1/-2/... before the extension when the name is taken.
func reserveUploadName(dir, name string) (string, error) {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for index := 0; index < 1000; index++ {
		candidate := name
		if index > 0 {
			candidate = fmt.Sprintf("%s-%d%s", base, index, ext)
		}
		target := filepath.Join(dir, candidate)
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			file.Close()
			return target, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", errors.New("无法写入项目目录，请检查磁盘权限")
		}
	}
	return "", errors.New("同名文件过多，请重命名后再上传")
}

// uploadLimitText renders a byte limit for the readable Chinese error message.
func uploadLimitText(limit int64) string {
	const mib, kib = 1 << 20, 1 << 10
	switch {
	case limit%mib == 0:
		return fmt.Sprintf("%d MiB", limit/mib)
	case limit%kib == 0:
		return fmt.Sprintf("%d KiB", limit/kib)
	default:
		return fmt.Sprintf("%d 字节", limit)
	}
}

// uploadError answers with {"error":"..."} so the web UI can show the Chinese
// message directly through its normal failure toast.
func uploadError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
