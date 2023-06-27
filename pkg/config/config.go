package config

import (
	"flag"
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"os"
)

const DEFAULT_CONFIG_FILE_NAME = "config.yaml"

func LoadConfig() *UpdateProxyConfig {
	configFile := configFileName()
	if len(configFile) > 0 {
		_, err := os.Stat(configFile)
		if err != nil {
			log.Fatalf("cannot find given config file: %s", configFile)
		}
	} else {
		_, err := os.Stat(DEFAULT_CONFIG_FILE_NAME)
		if err == nil {
			configFile = DEFAULT_CONFIG_FILE_NAME
		}
	}

	var config UpdateProxyConfig
	var err error
	if len(configFile) > 0 {
		log.Printf("Load Config from File: %s", configFile)
		err = cleanenv.ReadConfig(configFile, &config)
	} else {
		log.Print("Load Config from Environment")
		err = cleanenv.ReadEnv(&config)
	}

	if err != nil {
		log.Fatal(err)
	}

	return &config
}

func configFileName() string {
	configFile := flag.String("config-file", "", "Name or path of configuration file")
	flag.Parse()

	configFileEnv, ok := os.LookupEnv("CONFIG_FILE")
	if ok {
		return configFileEnv
	}

	return *configFile
}
