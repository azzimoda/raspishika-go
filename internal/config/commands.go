package config

type CommandsConfig struct {
	MainBot  []map[string]string `yaml:"mainbot"`
	AdminBot []map[string]string `yaml:"adminbot"`
}
