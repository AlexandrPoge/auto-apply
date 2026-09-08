package config

type Config struct {
	AppName string
}

func NewConfig() *Config {
	return &Config{
		AppName: "HH auto apply",
	}
}
