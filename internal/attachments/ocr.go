package attachments

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func ExtractTextFromImage(filePath string) string {
	if filePath == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if runtime.GOOS == "windows" {
		return extractWindowsOCR(ctx, filePath)
	}

	return ""
}

func extractWindowsOCR(ctx context.Context, filePath string) string {
	psScript := `
$ErrorActionPreference = 'SilentlyContinue'
Add-Type -AssemblyName System.Drawing
[Windows.Media.Ocr.OcrEngine, Windows.Foundation.UniversalApiContract, ContentType = WindowsRuntime] | Out-Null
[Windows.Graphics.Imaging.BitmapDecoder, Windows.Foundation.UniversalApiContract, ContentType = WindowsRuntime] | Out-Null
[Windows.Storage.StorageFile, Windows.Foundation.UniversalApiContract, ContentType = WindowsRuntime] | Out-Null

$path = [System.IO.Path]::GetFullPath($args[0])
$file = [Windows.Storage.StorageFile]::GetFileFromPathAsync($path).GetAwaiter().GetResult()
if (-not $file) { exit 0 }
$stream = $file.OpenAsync([Windows.Storage.FileAccessMode]::Read).GetAwaiter().GetResult()
if (-not $stream) { exit 0 }
$decoder = [Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream).GetAwaiter().GetResult()
$bitmap = $decoder.GetSoftwareBitmapAsync().GetAwaiter().GetResult()
$engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
if (-not $engine) { $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage([Windows.Globalization.Language]::new('en-US')) }
if (-not $engine) { exit 0 }
$result = $engine.RecognizeAsync($bitmap).GetAwaiter().GetResult()
if ($result -and $result.Text) {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    Write-Output $result.Text
}
`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", psScript, filePath)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
