package config

type Config struct {
	AppName string
	HHToken string
	SearchText string
	SearchArea string
	ApplyInterval time.Duration
}

func NewConfig() (*Config, error) {
	return &Config{
		AppName: "HH auto apply",
	}, nil
}