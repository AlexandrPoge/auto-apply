package main

import (
	"fmt"
	"hh-autoapply/internal/app"
	"hh-autoapply/internal/config"
	"hh-autoapply/internal/model"
)

func main() {
	cfg, err := config.NewConfig()
	if err != nil {
		panic(err)
	}

	app := app.NewApplication(cfg)
	app.Run()

	counter := &model.Counter{}
	counter.Increment()
	fmt.Println(counter.Value)
}
