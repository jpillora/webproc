package main

import (
	"log"
	"os"

	"github.com/jpillora/opts"
	"github.com/jpillora/webproc/agent"
)

var VERSION = "0.0.0-src"

func main() {
	//prepare config!
	c := agent.Config{}
	//parse cli
	opts.New(&c).PkgRepo().Version(VERSION).Parse()
	//if args contains has one non-executable file, treat as webproc file
	//TODO: allow cli to override config file
	args := c.ProgramArgs
	if len(args) == 1 {
		path := args[0]
		if info, err := os.Stat(path); err == nil && info.Mode()&0111 == 0 {
			c.ProgramArgs = nil
			if err := agent.LoadConfig(path, &c); err != nil {
				log.Fatalf("[webproc] load config error: %s", err)
			}
		}
	}
	//validate and apply defaults
	if err := agent.ValidateConfig(&c); err != nil {
		log.Fatalf("[webproc] load config error: %s", err)
	}
	//server listener
	if err := agent.Run(c); err != nil {
		log.Fatalf("[webproc] agent error: %s", err)
	}
}
