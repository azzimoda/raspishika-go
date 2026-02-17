package browser_test

import (
	"os"
	"testing"

	"github.com/azzimoda/raspishika-go/internal/services/browser"
)

func BenchmarkBrowserService_TakeScreenshotHTML(b *testing.B) {
	html := "<html><body><h1>Benchmark Test</h1></body></html>"
	tempDir := b.TempDir()

	bs, err := browser.New()
	if err != nil {
		b.Fatalf("failed to create browser service: %v", err)
	}

	b.StartTimer()
	b.SetBytes(int64(len(html)))

	for b.Loop() {
		tempFile, err := os.CreateTemp(tempDir, "*.png")
		if err != nil {
			b.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		if err := bs.TakeScreenshotHTML(html, tempFile.Name()); err != nil {
			b.Fatalf("failed to take screenshot: %v", err)
		}
	}

	b.StopTimer()
	b.ReportAllocs()
}
