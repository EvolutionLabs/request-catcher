package catcher

import (
	"encoding/json"
	"os"
)

type Configuration struct {
	HTTPPort      int `json:"http_port"`
	HTTPSPort     int `json:"https_port"`
	Host          string
	RootHost      string `json:"root_host"`
	TLSDir        string `json:"tls_dir"`
	User          string `json:"user"`
	Password      string `json:"password"`
	PostOnly      bool   `json:"post_only"`
	AllowMultiple bool   `json:"allow_multiple"`
}

func LoadConfiguration(filename string) (*Configuration, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(file)
	config := Configuration{}
	err = decoder.Decode(&config)

	return &config, err
}
