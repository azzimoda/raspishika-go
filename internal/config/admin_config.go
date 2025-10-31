package config

import (
	"encoding/json"
	"os"
)

// AdminConfig is the configuration for the admin bot.
// It can be updated dynamically by using the admin bot commands.
type AdminConfig struct {
	NewChatReport bool `json:"new_chat_report"`
	DailyReport   bool `json:"daily_report"`
}

func (c *AdminConfig) Save(filename string) error {
	configFile, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, configFile, 0644)
}

func LoadAdminConfig(filename string) (*AdminConfig, error) {
	configFile, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var adminConfig AdminConfig
	json.Unmarshal(configFile, &adminConfig)

	return &adminConfig, nil
}
