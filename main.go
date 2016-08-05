package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/jpillora/backoff"
	"github.com/naoina/toml"
)

var VERSION = "0.0.0-src"

var help = `
    Usage: webproc <toml.file>

    For more documentation, visit:
        https://github.com/jpillora/webproc

    Version:
        ` + VERSION + `

`

type (
	Log      string
	OnExit   string
	Duration time.Duration
)

const (
	LogBoth  Log = "both"
	LogWebUI     = "webui"
	LogProxy     = "proxy"

	OnExitRestart = "restart"
	OnExitIgnore  = "ignore"
	OnExitProxy   = "proxy"
)

type Config struct {
	Host               string
	Port               int
	User, Pass         string
	Program            string
	Arguments          []string
	Log                Log
	OnExit             OnExit
	ConfigurationFiles []string
	VerifyProgram      string
	VerifyArguments    []string
	RestartSignal      string
	RestartTimeout     Duration
}

func main() {
	//super complex cli parser
	if len(os.Args) < 2 {
		os.Stdout.WriteString(help)
		os.Exit(0)
	}
	if v := os.Args[1]; v == "-v" || v == "--version" {
		os.Stdout.WriteString(VERSION)
		os.Exit(0)
	}
	//config file reader
	path := os.Args[1]
	if info, err := os.Stat(path); os.IsNotExist(err) {
		fatalf("file not found")
	} else if err != nil {
		fatalf("file error: %s", err)
	} else if info.IsDir() {
		fatalf("file not specified")
	}
	cbuff, err := ioutil.ReadFile(path)
	if err != nil {
		fatalf("file read error: %s", err)
	}
	//toml reader
	c := Config{}
	if err := toml.Unmarshal(cbuff, &c); err != nil {
		fatalf("toml syntax error: %s", err)
	}
	//config check
	prog, err := exec.LookPath(c.Program)
	if err != nil {
		fatalf("program not found in path: %s", err)
	}
	//server listener
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		fatalf("failed to start server: %s", err)
	}
	//create agent
	a := newAgent(c)
	//serve on another thread (only returns on error)
	go func() {
		err := http.Serve(l, a.root)
		fatalf("http error: %s", err)
	}()
	//report server address
	a.logf("agent listening on http://%s:%d...", c.Host, c.Port)
	//program run loop
	b := backoff.Backoff{}
	for {
		//start proc
		proc := exec.Command(prog, c.Arguments...)
		a.setProc(proc)
		if err := proc.Start(); err != nil {
			fatalf("program failed to start: %s", err)
		}
		//block here while running
		err := proc.Wait()
		//check exit code
		code := 0
		if err != nil {
			code = 1
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					code = status.ExitStatus()
				}
			}
		}
		//exit expected?
		if !a.expectExit {
			os.Exit(code)
		}
		delay := b.Duration()
		a.debugf("program exited with %d, restart in %s...", code, delay)
		//delay then restart
		time.Sleep(delay)
	}
}

func fatalf(format string, args ...interface{}) {
	log.Fatalf("[webproc] "+format, args...)
}

//=======================================
// helper types

func (o *OnExit) UnmarshalTOML(data []byte) error {
	*o = OnExit(quoted(data))
	return nil
}

func (o *Log) UnmarshalTOML(data []byte) error {
	*o = Log(quoted(data))
	return nil
}

func (d *Duration) UnmarshalTOML(data []byte) error {
	d2, err := time.ParseDuration(quoted(data))
	*d = Duration(d2)
	return err
}

func quoted(data []byte) string {
	if l := len(data); l >= 2 {
		return string(data[1 : l-1])
	}
	return string(data)
}
