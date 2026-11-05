package config

import "github.com/ihezebin/olympus/logger"

type LoggerConfig struct {
	Level    logger.Level `json:"level"`
	Filename string       `json:"filename"`
}
type Config struct {
	ServiceName string        `json:"service_name"`
	Pwd         string        `json:"-"`
	Logger      *LoggerConfig `json:"logger"`
	Port        uint
	Proxy       string
	Cookie      string
	CookiesFile string
	WebDir      string
	APIOnly     bool
}
