package zalo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	maxUploadBytes  = int64(45 * 1024 * 1024)
	maxFilesPerTurn = 5
	maxMessageChars = 1800
)

var (
	markdownTargetRE = regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)
	windowsPathRE    = regexp.MustCompile(`[A-Za-z]:[/\\][^\r\n"<>|?*\x60()\[\]]+`)
	linkMarkdownRE   = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	imageMarkdownRE  = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`)
)

var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true,
	".gif": true, ".bmp": true,
}

var sendableExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".bmp": true,
	".pdf": true, ".xlsx": true, ".xls": true, ".csv": true, ".docx": true, ".doc": true,
	".pptx": true, ".ppt": true, ".txt": true, ".zip": true, ".mp4": true,
}

func isImageFile(path string) bool {
	return imageExtensions[strings.ToLower(filepath.Ext(path))]
}

func isSendableFile(path string) bool {
	if !sendableExtensions[strings.ToLower(filepath.Ext(path))] {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0 && info.Size() <= maxUploadBytes
}

func listOutputFiles(workspace string) []string {
	if workspace == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(workspace, "output"))
	if err != nil {
		return nil
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			path := filepath.Join(workspace, "output", entry.Name())
			if isSendableFile(path) {
				files = append(files, path)
			}
		}
	}
	sort.Strings(files)
	return files
}

func resolveOutboundFile(workspace, argument string) string {
	raw := strings.Trim(strings.TrimSpace(argument), `"'`)
	if raw == "" {
		return ""
	}
	candidates := []string{raw}
	if workspace != "" {
		candidates = append(candidates,
			filepath.Join(workspace, raw),
			filepath.Join(workspace, "output", raw),
			filepath.Join(workspace, "input", raw),
		)
	}
	for _, candidate := range candidates {
		if absolute, err := filepath.Abs(candidate); err == nil && isSendableFile(absolute) {
			return absolute
		}
	}
	needle := strings.ToLower(raw)
	for _, path := range listOutputFiles(workspace) {
		if strings.Contains(strings.ToLower(filepath.Base(path)), needle) {
			return path
		}
	}
	return ""
}

func extractMediaPaths(text, workspace string, since time.Time) []string {
	candidates := make([]string, 0, 8)
	for _, match := range markdownTargetRE.FindAllStringSubmatch(text, -1) {
		if len(match) > 1 {
			candidates = append(candidates, match[1])
		}
	}
	candidates = append(candidates, windowsPathRE.FindAllString(text, -1)...)
	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, maxFilesPerTurn)
	for _, candidate := range candidates {
		candidate = strings.Trim(strings.TrimSpace(candidate), `"'`)
		candidate = strings.TrimPrefix(candidate, "file://")
		if !filepath.IsAbs(candidate) && workspace != "" {
			candidate = filepath.Join(workspace, candidate)
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil || !isSendableFile(absolute) {
			continue
		}
		info, err := os.Stat(absolute)
		if err != nil || (!since.IsZero() && info.ModTime().Before(since)) {
			continue
		}
		key := strings.ToLower(filepath.Clean(absolute))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, absolute)
		if len(out) == maxFilesPerTurn {
			break
		}
	}
	return out
}

func sanitizeReply(text string) string {
	text = imageMarkdownRE.ReplaceAllString(text, "")
	text = linkMarkdownRE.ReplaceAllString(text, "$1")
	text = windowsPathRE.ReplaceAllString(text, "")
	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "đường dẫn file:") || strings.HasPrefix(lower, "file path:") || strings.HasPrefix(lower, "saved to:") {
			continue
		}
		out = append(out, strings.TrimRight(line, " \t"))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func chunkText(text string, limit int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	parts := make([]string, 0, (len(runes)/limit)+1)
	for len(runes) > limit {
		cut := limit
		for i := limit; i > limit/2; i-- {
			if runes[i-1] == '\n' || runes[i-1] == ' ' {
				cut = i
				break
			}
		}
		parts = append(parts, strings.TrimSpace(string(runes[:cut])))
		runes = runes[cut:]
	}
	if tail := strings.TrimSpace(string(runes)); tail != "" {
		parts = append(parts, tail)
	}
	return parts
}

func captureScreenshot(ctx context.Context, output string) error {
	if runtime.GOOS != "windows" {
		return errors.New("chụp màn hình từ xa hiện chỉ hỗ trợ Windows")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	escaped := strings.ReplaceAll(output, "'", "''")
	script := `$code=@"
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;
public class ScreenCap {
 [DllImport("user32.dll")] public static extern IntPtr GetDesktopWindow();
 [DllImport("user32.dll")] public static extern IntPtr GetWindowDC(IntPtr hWnd);
 [DllImport("user32.dll")] public static extern IntPtr ReleaseDC(IntPtr hWnd, IntPtr hDC);
 [DllImport("gdi32.dll")] public static extern bool BitBlt(IntPtr d,int x,int y,int w,int h,IntPtr s,int sx,int sy,int op);
 [DllImport("gdi32.dll")] public static extern IntPtr CreateCompatibleBitmap(IntPtr d,int w,int h);
 [DllImport("gdi32.dll")] public static extern IntPtr CreateCompatibleDC(IntPtr d);
 [DllImport("gdi32.dll")] public static extern bool DeleteDC(IntPtr d);
 [DllImport("gdi32.dll")] public static extern bool DeleteObject(IntPtr o);
 [DllImport("gdi32.dll")] public static extern IntPtr SelectObject(IntPtr d,IntPtr o);
 [DllImport("user32.dll")] public static extern int GetSystemMetrics(int i);
 [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
 public static void Capture(string path) {
  SetProcessDPIAware(); int w=GetSystemMetrics(0), h=GetSystemMetrics(1);
  IntPtr desk=GetDesktopWindow(), src=GetWindowDC(desk), dst=CreateCompatibleDC(src);
  IntPtr bmp=CreateCompatibleBitmap(src,w,h), old=SelectObject(dst,bmp);
  BitBlt(dst,0,0,w,h,src,0,0,0x00CC0020); SelectObject(dst,old); DeleteDC(dst); ReleaseDC(desk,src);
  using (Bitmap image=Image.FromHbitmap(bmp)) { image.Save(path,ImageFormat.Png); }
  DeleteObject(bmp);
 }
}
"@; Add-Type -TypeDefinition $code -ReferencedAssemblies System.Drawing; [ScreenCap]::Capture('` + escaped + `')`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	hideConsoleWindow(cmd)
	if outputBytes, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("lỗi khi chụp màn hình: %w: %s", err, strings.TrimSpace(string(outputBytes)))
	}
	if !isSendableFile(output) {
		return errors.New("không tìm thấy ảnh chụp màn hình sau khi thực hiện")
	}
	return nil
}

func uploadFile(ctx context.Context, path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {

		return "", fmt.Errorf("Không đọc được tệp %s: %w", path, err)
	}
	if info.Size() == 0 {

		return "", fmt.Errorf("Tệp rỗng: %s", path)
	}
	if info.Size() > maxUploadBytes {

		return "", fmt.Errorf("Tệp %s nặng %.1f MB, vượt giới hạn 45 MB", path, float64(info.Size())/(1024*1024))
	}
	name := filepath.Base(path)
	client := &http.Client{Timeout: 120 * time.Second}
	type provider struct {
		name, endpoint, field string
		values                map[string]string
		parse                 func([]byte) (string, error)
	}
	plainURL := func(data []byte) (string, error) {
		value := strings.TrimSpace(string(data))
		if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
			return value, nil
		}
		return "", fmt.Errorf("phản hồi không hợp lệ: %s", value)
	}
	providers := []provider{
		{name: "Litterbox", endpoint: "https://litterbox.catbox.moe/resources/internals/api.php", field: "fileToUpload", values: map[string]string{"reqtype": "fileupload", "time": "72h"}, parse: plainURL},
		{name: "Tmpfiles", endpoint: "https://tmpfiles.org/api/v1/upload", field: "file", parse: func(data []byte) (string, error) {
			var payload struct {
				Data struct {
					URL string `json:"url"`
				} `json:"data"`
			}
			if json.Unmarshal(data, &payload) != nil || payload.Data.URL == "" {
				return "", errors.New("không tìm thấy URL trong phản hồi")
			}
			trimmed := strings.TrimPrefix(strings.TrimPrefix(payload.Data.URL, "https://tmpfiles.org/"), "http://tmpfiles.org/")
			return "https://tmpfiles.org/dl/" + strings.TrimPrefix(trimmed, "dl/"), nil
		}},
		{name: "0x0.st", endpoint: "https://0x0.st", field: "file", parse: plainURL},
		{name: "File.io", endpoint: "https://file.io", field: "file", parse: func(data []byte) (string, error) {
			var payload struct {
				Link string `json:"link"`
			}
			if json.Unmarshal(data, &payload) != nil || payload.Link == "" {
				return "", errors.New("không có trường link trong phản hồi")
			}
			return payload.Link, nil
		}},
	}
	if blockedLitterboxExtension(name) {
		providers = providers[1:]
	}
	errorsByProvider := make([]string, 0, len(providers))
	for _, candidate := range providers {
		data, err := postMultipartFile(ctx, client, candidate.endpoint, candidate.field, path, candidate.values)
		if err == nil {
			if direct, parseErr := candidate.parse(data); parseErr == nil {
				return direct, nil
			} else {
				err = parseErr
			}
		}
		errorsByProvider = append(errorsByProvider, candidate.name+": "+err.Error())
	}
	return "", fmt.Errorf("không thể tải tệp lên dịch vụ chia sẻ đám mây: %s", strings.Join(errorsByProvider, "; "))
}

func blockedLitterboxExtension(name string) bool {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(name), ".")) {
	case "exe", "scr", "cpl", "jar", "doc", "docx", "docm":
		return true
	default:
		return false
	}
}

func postMultipartFile(ctx context.Context, client *http.Client, endpoint, field, path string, values map[string]string) ([]byte, error) {
	reader, writer := io.Pipe()
	form := multipart.NewWriter(writer)
	go func() {
		var writeErr error
		defer func() {
			if writeErr == nil {
				writeErr = form.Close()
			}
			_ = writer.CloseWithError(writeErr)
		}()
		for key, value := range values {
			if writeErr = form.WriteField(key, value); writeErr != nil {
				return
			}
		}
		file, err := os.Open(path)
		if err != nil {
			writeErr = err
			return
		}
		defer file.Close()
		part, err := form.CreateFormFile(field, filepath.Base(path))
		if err != nil {
			writeErr = err
			return
		}
		_, writeErr = io.Copy(part, file)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reader)
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}
