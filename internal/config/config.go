package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Configuration struct {
	Address      string
	ReadTimeout  int
	WriteTimeout int
}

var Config *Configuration

func init() {
	configPath, _ := filepath.Abs(filepath.Join("config.json"))
	envPath, _ := filepath.Abs(filepath.Join(".env"))

	// load .env file
	err := godotenv.Load(envPath)
	if err != nil {
		log.Printf("WARNING %v", err)
	}

	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalln("Cannot open config file", err)
	}
	decoder := json.NewDecoder(file)
	Config = &Configuration{}
	err = decoder.Decode(Config)
	if err != nil {
		log.Fatalln("Cannot get configuration from file", err)
	}
}
