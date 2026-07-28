package main

import (
	"fmt"
	"hh-auto-apply/internal/config"
)

func main() {
	cfg := config.NewConfig()
	fmt.Println(cfg.AppName)
}