package main

import (
	"os"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

type Configuration struct {
	KeyMatchers      []string          `yaml:"defaultKeyMatchers"`
	ProviderMatchers []ProviderMatcher `yaml:"providerMatchers"`
	TypeMatchers     []TypeMatcher     `yaml:"typeMatchers"`
}

type TypeMatcher struct {
	Names           []string        `yaml:"names"`
	ProviderMatcher ProviderMatcher `yaml:"providerMatcher"`
}

type ProviderMatcher struct {
	Names       []string `yaml:"names"`
	KeyMatchers []string `yaml:"keyMatchers"`
}

func (c *Configuration) LoadConfiguration(confPath string) {
	yamlFile, err := os.ReadFile(confPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't read config file")
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatal().Err(err).Msg("Unmarshal error")
	}
	log.Debug().Interface("Base configuration", c).Send()
}
