package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/logger"
)

func InitPaths() {
	_, filename, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(filepath.Dir(filename), "..", "..", "..")

	viper.Set("config_file", filepath.Join(rootDir, "configs/.debug-config.yml"))
	viper.Set("commands_file", filepath.Join(rootDir, "configs/commands.yml"))
	viper.Set("schedule_template_file", filepath.Join(rootDir, "storage/schedule_template.html"))
}

// TODO: Adapt to new services structure.
func InitServices(t *testing.T, debugConfigFile, templateFile string) (
	string,
	*database.Repository,
	*browser.BrowserService,
	*cache.Cache,
) {
	testsDir := filepath.Join(os.TempDir(), time.Now().Format("20060102150405"))
	if err := os.MkdirAll(testsDir, 0755); err != nil {
		t.Fatalf("could not create tests directory: %v", err)
	}

	log.Trace().Str("debugConfigFile", debugConfigFile).Str("templateFile", templateFile).Str("testsDir", testsDir).
		Msg("InitServices")

	InitConfig(t, testsDir)

	repo, err := database.New()
	if err != nil {
		t.Fatalf("could not construct repository: %v", err)
	}

	browserService, err := browser.New()
	if err != nil {
		t.Fatalf("could not construct browser: %v", err)
	}

	cacheService := cache.New()

	return testsDir, repo, browserService, cacheService
}

func InitConfig(t *testing.T, testsDir string) {
	if err := config.Load(); err != nil {
		t.Fatalf("could not load config: %v", err)
	}

	viper.Set("database.file", filepath.Join(testsDir, "database/test.sqlite3"))
	viper.Set("browser.screenshot_dir", filepath.Join(testsDir, "storage/screenshots"))
	viper.Set("cache.dir", filepath.Join(testsDir, "storage/cache"))
	viper.Set("logger.dir", "")

	logger.SetupLogger("trace", "")

	if err := config.EnsureDirs(); err != nil {
		t.Fatalf("could not ensure dirs: %v", err)
	}
}

// cleanup removes all files and directories created during testing.
func Cleanup(t *testing.T, testsDir string) {
	if err := os.RemoveAll(testsDir); err != nil {
		t.Logf("could not delete tests dir: %v", err)
	}
}
