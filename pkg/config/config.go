package config

import (
	"errors"
	"flag"
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"os"
	"strings"
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

func (config *UpdateProxyConfig) Validate() error {
	if len(config.Upstreams) == 0 {
		return errors.New("cannot find any upstream endpoint")
	}

	for i, upstream := range config.Upstreams {
		path := strings.ToLower(upstream.Path)
		for j, inner := range config.Upstreams {
			if i == j {
				continue
			}

			if strings.Compare(path, strings.ToLower(inner.Path)) == 0 {
				return errors.New("duplicate paths detected")
			}
		}
	}

	return nil
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
