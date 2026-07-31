package app

import (
	"fmt"
	"hh-autoapply/internal/config"
)

type Apllication struct {
	Config *config.Config
}

func NewApplication(config *config.Config) *Apllication {
	return &Apllication{
		Config: config,
	}
}

func (a *Apllication) Run() {
	fmt.Println(a.Config.AppName)
}