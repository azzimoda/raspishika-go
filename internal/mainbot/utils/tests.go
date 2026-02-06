package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/logger"
)

// InitPaths initializes the paths for the tests. Should be called before any other function in this package.
func InitPaths() {
	_, filename, _, _ := runtime.Caller(0)
	rootDir := filepath.Join(filepath.Dir(filename), "..", "..", "..")

	viper.Set("root", rootDir)
	viper.Set("config_file", filepath.Join(rootDir, "configs/.debug-config.yml"))
	viper.Set("commands_file", filepath.Join(rootDir, "configs/commands.yml"))
	viper.Set("schedule_template_file", filepath.Join(rootDir, "storage/schedule_template.html"))
	viper.Set("database.migrations", filepath.Join(rootDir, "database/migrations"))
}

func InitConfig(t *testing.T, testsDir string) {
	viper.Set("env", filepath.Join(viper.GetString("root"), ".env"))
	viper.Set("database.file", filepath.Join(testsDir, "database/test.sqlite3"))
	viper.Set("browser.screenshot_dir", filepath.Join(testsDir, "storage/screenshots"))
	viper.Set("cache.dir", filepath.Join(testsDir, "storage/cache"))
	viper.Set("logger.dir", "")

	if err := config.Load(); err != nil {
		t.Fatalf("could not load config: %v", err)
	}

	logger.SetupLogger("trace", "")

	if err := config.MkDirs(); err != nil {
		t.Fatalf("could not ensure dirs: %v", err)
	}
}

// cleanup removes all files and directories created during testing.
func Cleanup(t *testing.T, testsDir string) {
	if err := os.RemoveAll(testsDir); err != nil {
		t.Logf("could not delete tests dir: %v", err)
	}
}
