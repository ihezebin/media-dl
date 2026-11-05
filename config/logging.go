package config

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// NormalizeLogging gives every service a distinct file in the shared log directory.
func (c *Config) NormalizeLogging() error {
	if c.ServiceName == "" {
		c.ServiceName = "media-dl"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`).MatchString(c.ServiceName) {
		return fmt.Errorf("invalid service_name")
	}
	if c.Logger == nil {
		c.Logger = &LoggerConfig{}
	}
	if c.Logger.Level == "" {
		c.Logger.Level = "info"
	}
	c.Logger.Filename = filepath.Join("log", c.ServiceName+".log")
	return nil
}
