package app

import (
	"fmt"
	"hh-autoapply/internal/config"
)

type Apllication struct {
	Config *config.Config
}

func NewApplication(cfg *config.Config) *Apllication {
	return &Apllication{
		Config: cfg,
	}
}

func (a *Apllication) Run() {
	fmt.Println(a.Config.AppName)
}
