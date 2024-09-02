package quote_engine

import (
	"os"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type LogConfig struct {
	Dir          string            `yaml:"dir"`
	LinkName     string            `yaml:"link_name"`
	Level        string            `yaml:"level"`
	ReportCaller bool              `yaml:"report_caller"`
	Format       string            `yaml:"format"`
	Writers      []WriterConfig    `yaml:"writer"`
	WriteMap     map[string]string `yaml:"write_map"`
}

type WriterConfig struct {
	Name         string `yaml:"name"`
	Path         string `yaml:"path"`
	MaxAge       int    `yaml:"max_age"`
	RotationTime int    `yaml:"rotation_time"`
}

type PublisherConfig struct {
	Topic string `yaml:"topic"`
	Skey  int    `yaml:"skey"`
	Size  int    `yaml:"size"`
}

type WsClientConfig struct {
	Exchange   string            `yaml:"exchange"`
	Url        string            `yaml:"url"`
	Subscribe  []string          `yaml:"subscribe"`
	ReconnTime int               `yaml:"reconn_time"`
	CMD        []WsAPIConfig     `yaml:"cmd"`
	Publisher  []PublisherConfig `yaml:"publisher"`
}

type WsAPIConfig struct {
	Method string            `yaml:"method"`
	Params map[string]string `yaml:"params"`
}

type APIClientConfig struct {
	Exchange string            `yaml:"exchange"`
	Url      string            `yaml:"url"`
	EndPoint string            `yaml:"endpoint"`
	Query    map[string]string `yaml:"queries"`
	Param    map[string]string `yaml:"params"`
}

type Config struct {
	Log       LogConfig        `yaml:"Log"`
	Websocket []WsClientConfig `yaml:"Websocket"`
	APIClient APIClientConfig  `yaml:"APIClient"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open("config.yaml")
	if err != nil {
		logrus.Println(err)
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(config)
	if err != nil {
		logrus.Println(err)
		return nil, err
	}
	return config, nil
}
