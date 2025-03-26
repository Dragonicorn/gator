package config

import (
	"encoding/json"
	"log"
	"os"
)

const configFileName = ".gatorconfig.json"

type Config struct {
	DbURL    string `json:"db_url"`
	UserName string `json:"current_user_name"`
}

func getConfigFilePath() (string, error) {
	hd, err := os.UserHomeDir()
	return hd + "/" + configFileName, err
}

func Read() Config {
	fn, err := getConfigFilePath()
	if err != nil {
		log.Fatal(err)
	}
	text, err := os.ReadFile(fn)
	if err != nil {
		log.Fatal(err)
	}
	config := Config{}
	err = json.Unmarshal(text, &config)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func (cfg *Config) SetUser(user string) {
	cfg.UserName = user
	text, err := json.Marshal(cfg)
	if err != nil {
		log.Fatal(err)
	}
	fn, err := getConfigFilePath()
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(fn, text, 0o644)
	if err != nil {
		log.Fatal(err)
	}
}
