package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const configFileName = ".gatorconfig.json"

func getConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(homeDir, configFileName)
	return configPath, nil
}

func Write(cfg Config) error {
	configPath, err := getConfigFilePath()
	if err != nil {
		return err
	}

	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		return err
	}

	return nil
}

type Config struct {
	DbUrl           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func (cfg *Config) Read() (Config, error) {
	configPath, err := getConfigFilePath()
	if err != nil {
		return Config{}, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg.CurrentUserName = "lane"
		if err := Write(*cfg); err != nil {
			return Config{}, err
		}
	}

	file, err := os.Open(configPath)
	if err != nil {
		return Config{}, err
	}

	defer file.Close()
	decoder := json.NewDecoder(file)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}
