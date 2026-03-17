package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/pkg/logger"
)

func init() {
	_, filename, _, _ := runtime.Caller(0)

	// Find project's root rootDir.
	rootDir := filepath.Dir(filename)
	for !isProjectRootDir(rootDir) && rootDir != "/" {
		rootDir = filepath.Dir(rootDir)
	}
	if rootDir == "/" {
		panic("unable to find project root directory")
	}
	log.Trace().Str("projectRootDir", rootDir).Msg("Initializing paths...")

	configFile := "configs/.debug-config.yml"
	if _, err := os.Stat(configFile); err != nil {
		configFile = viper.GetString(config.KeyConfigFile)
	}

	viper.Set(config.KeyDotEnv, filepath.Join(rootDir, ".env"))
	viper.Set(config.KeyConfigFile, filepath.Join(rootDir, "configs/config.yml"))
	viper.Set(config.KeyCommandsFile, filepath.Join(rootDir, "configs/commands.yml"))
	viper.Set(config.KeyScheduleTemplateFile, filepath.Join(rootDir, "storage/schedule_template.html"))
	viper.Set(config.KeyDatabaseMigrations, filepath.Join(rootDir, "migrations"))
	log.Trace().Msg("Configured paths")
}

// isProjectRootDir checks if the given directory is the root directory of the project.
//
// A directory is considered the root directory of the project if it contains a go.mod file.
func isProjectRootDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

func InitConfig(t *testing.T, testsDir string) {
	viper.Set(config.KeyDatabaseFile, filepath.Join(testsDir, "database/test.sqlite3"))
	viper.Set(config.KeyBrowserScreenshotDir, filepath.Join(testsDir, "storage/screenshots"))
	viper.Set(config.KeyCacheDir, filepath.Join(testsDir, "storage/cache"))
	viper.Set(config.KeyLoggerDir, "")

	if err := config.Load(); err != nil {
		t.Fatalf("could not load config: %v", err)
	}
	logger.SetupLogger("trace", "")
}

// cleanup removes all files and directories created during testing.
func Cleanup(t *testing.T, testsDir string) {
	if err := os.RemoveAll(testsDir); err != nil {
		t.Logf("could not delete tests dir: %v", err)
	}
}
