package main

import (
	"flag"
	"github.com/jolestar/go-probe/pkg/web"
	"log"
	"os"
)

func main() {
	flag.Parse()
	log.Print("Starting go-probe")
	config := &web.Config{Listen: ":8080"}
	probe, err := web.New(config)
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(-1)
	}
	probe.Init()
	probe.Serve()
}
