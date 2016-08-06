package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/jpillora/opts"
	"github.com/jpillora/webproc/wp"
)

var VERSION = "0.0.0-src"

func main() {
	//prepare config!
	c := wp.Config{}
	//parse cli
	opts.New(&c).PkgRepo().Version(VERSION).Parse()
	//if args contains has one non-executable file, treat as webproc file
	//TODO: allow cli to override config file
	args := c.ProgramArgs
	if len(args) == 1 {
		path := args[0]
		if info, err := os.Stat(path); err == nil && info.Mode()&0111 == 0 {
			c.ProgramArgs = nil
			if err := wp.LoadConfig(path, &c); err != nil {
				fatalf("load config error: %s", err)
			}
		}
	}
	//validate and apply defaults
	if err := wp.ValidateConfig(&c); err != nil {
		fatalf("load config error: %s", err)
	}
	//server listener
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		fatalf("failed to start server: %s", err)
	}
	//create agent
	a := wp.NewAgent(c)
	//serve agent's root handler on another thread
	go func() {
		err := http.Serve(l, a)
		fatalf("http error: %s", err)
	}()
	//run process
	wp.RunProcess(c, a)
}

func fatalf(format string, args ...interface{}) {
	log.Fatalf("[webproc] "+format, args...)
}
