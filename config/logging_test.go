package config

import "testing"

func TestNormalizeLogging(t *testing.T) {
	c := &Config{ServiceName: "custom-service"}
	if err := c.NormalizeLogging(); err != nil {
		t.Fatal(err)
	}
	if c.Logger.Filename != "log/custom-service.log" || c.Logger.Level != "info" {
		t.Fatal("wrong log defaults")
	}
	c.ServiceName = "../escape"
	if err := c.NormalizeLogging(); err == nil {
		t.Fatal("unsafe service name accepted")
	}
}
