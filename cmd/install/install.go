package main

import (
	"log"

	"github.com/go-rod/rod/lib/launcher"
)

func main() {
	err := launcher.NewBrowser().Download()
	if err != nil {
		log.Fatalln(err)
	}
}
