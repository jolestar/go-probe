package main

import (
	"flag"
	"github.com/jolestar/go-probe/pkg/web"
	"log"
	"os"
)
var(
	listen string
)

func init()  {
	flag.StringVar(&listen, "listen", ":80", "Address to listen to (TCP)")
}

func main() {
	flag.Parse()
	log.Print("Starting go-probe")
	config := &web.Config{Listen: listen}
	probe, err := web.New(config)
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(-1)
	}
	probe.Init()
	probe.Serve()
}
