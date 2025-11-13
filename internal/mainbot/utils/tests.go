package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rs/zerolog/log"

	"github.com/azzimoda/raspishika-go/internal/browser"
	"github.com/azzimoda/raspishika-go/internal/cache"
	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/database"
	"github.com/azzimoda/raspishika-go/pkg/logger"
)

func InitPaths() (string, string, string) {
	_, filename, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(filepath.Dir(filename), "..", "..", "..")

	debugConfigFile := filepath.Join(rootDir, "configs/.debug-config.yml")
	templateFile := filepath.Join(rootDir, "storage/schedule_template.html")

	testsDir := filepath.Join(rootDir, ".tests")
	// Delete and recreate tests dir.
	if err := os.RemoveAll(testsDir); err != nil {
		log.Fatal().Err(err).Msg("could not delete tests dir")
	}
	if err := os.MkdirAll(testsDir, 0755); err != nil {
		log.Fatal().Err(err).Msg("could not create tests dir")
	}

	return debugConfigFile, templateFile, testsDir
}

// TODO: Use t.TempDir() instead of my manual implementation.
func InitServices(t *testing.T, debugConfigFile, templateFile, testsDir string) (
	*config.MainConfig,
	*database.Repository,
	*browser.BrowserService,
	*cache.Cache,
) {
	log.Trace().Str("debugConfigFile", debugConfigFile).Str("templateFile", templateFile).Str("testsDir", testsDir).
		Msg("InitServices")

	cfg := InitConfig(t, debugConfigFile, templateFile, testsDir)

	repo, err := database.New(cfg)
	if err != nil {
		t.Fatalf("could not construct repository: %v", err)
	}

	browserService, err := browser.New(cfg)
	if err != nil {
		t.Fatalf("could not construct browser: %v", err)
	}

	cacheService := cache.New(&cfg.Cache)

	return cfg, repo, browserService, cacheService
}

func InitConfig(t *testing.T, debugConfigFile, templateFile, testsDir string) *config.MainConfig {
	cfg, err := config.LoadMainConfig(debugConfigFile)
	if err != nil {
		t.Fatalf("could not load config: %v", err)
	}
	cfg.Database.File = filepath.Join(testsDir, "database/test.sqlite3")
	cfg.Browser.ScreenshotDir = filepath.Join(testsDir, "storage/screenshots")
	cfg.Cache.Dir = filepath.Join(testsDir, "storage/cache")
	cfg.Logger.Dir = "" // Disable logging to file for tests.
	cfg.ScheduleTemplateFile = templateFile
	logger.SetupLogger(config.LoggerConfig{Level: "trace", Dir: ""})

	log.Trace().Any("config", cfg).Msg("Loaded config")

	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("could not ensure dirs: %v", err)
	}

	if err := cfg.LoadTemplate(); err != nil {
		t.Fatalf("could not load template: %v", err)
	}

	// commandsCfg, err := config.LoadCommandsConfig(commandsConfigFile)
	// if err != nil {
	// 	t.Fatalf("could not load config: %v", err)
	// }

	return cfg
}

// cleanup removes all files and directories created during testing.
func Cleanup(t *testing.T, testsDir string) {
	if err := os.RemoveAll(testsDir); err != nil {
		t.Logf("could not delete tests dir: %v", err)
	}
}
