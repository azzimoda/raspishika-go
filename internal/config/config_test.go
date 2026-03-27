package config_test

import (
	"testing"

	"github.com/azzimoda/raspishika-go/internal/config"
	"github.com/azzimoda/raspishika-go/internal/testutil"
)

func TestLoadBotCommands(t *testing.T) {
	testDir := t.TempDir()
	testutil.InitConfig(t, testDir)

	err := config.LoadBotCommands()
	if err != nil {
		t.Errorf("loadBotCommands() error = %v", err)
	}

	mainBotCommands := config.MainBotCommands()
	adminBotCommands := config.AdminBotCommands()
	t.Logf("mainBotCommands: %v, adminBotCommands: %v", mainBotCommands, adminBotCommands)
	if len(mainBotCommands) == 0 || len(adminBotCommands) == 0 {
		t.Errorf("loadBotCommands() did not set any commands")
	}
}
