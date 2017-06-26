package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type DataBaseConfig struct {
	Dialect            string `json:"dialect"`
	Uri                string `json:"uri"`
	MaxIdleConnections int    `json:"max-idle-connections"`
	MaxOpenConnections int    `json:"max-open-connections"`
	ShowLog            bool   `json:"log"`
	Threads            uint8  `json:"threads"`
	Limit              uint16 `json:"limit"`
}

type S3Config struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`

	UploadThumbnailChannelSize uint `json:"upload_thumbnail_channel_size"`
	UploadOriginalChannelSize  uint `json:"upload_original_channel_size"`
}

type JWTConfig struct {
	SecretKey string `json:"secret_key"`
}

type CruftFlakeConfig struct {
	Uri string `json:"uri"`
}

type NewRelicConfig struct {
	AppName string `json:"appname"`
	Key     string `json:"key"`
}

type Configuration struct {
	NewRelic   NewRelicConfig   `json:"newrelic"`
	DB         DataBaseConfig   `json:"db"`
	S3         S3Config         `json:"s3"`
	CruftFlake CruftFlakeConfig `json:"cruftflake"`
	JWT        JWTConfig        `json:"jwt"`
}

func (this *Configuration) Init(configFile string) {
	configJson, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = json.Unmarshal(configJson, &this)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
