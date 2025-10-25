package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
)

const configFileName = ".gatorconfig.json"

type Config struct {
	DbURL string 			`json:"db_url"`
	CurrentUserName string 	`json:"current_user_name,omitempty"`
}

func Read() (Config, error) {
	filepath, err := getConfigFilePath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return Config{}, fmt.Errorf("Failed to read `%s`: %v\n", filepath, err)
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return Config{}, fmt.Errorf("Failed to unmarshal json: %v\n", err)
	}

	return config, nil
}

func (cfg Config) SetUser(userName string) error {
	cfg.CurrentUserName = userName

	err := write(cfg)
	return err
}

func getConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("Failed to get users home dir: %v\n", err)
	}

	filepath := path.Join(home, configFileName)

	return filepath, nil
}

func write(cfg Config) error {
	filepath, err := getConfigFilePath()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("Failed to marshal config: %v\n", err)
	}

	err = os.WriteFile(filepath, data, 0666)
	if err != nil {
		return fmt.Errorf("Failed to write to `%s`: %v\n", filepath, err)
	}

	return nil
}
