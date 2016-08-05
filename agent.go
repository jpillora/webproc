//go:generate go-bindata -pkg main -ignore .../.DS_Store -o agent_static.go static/...

package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/jpillora/cookieauth"
	"github.com/jpillora/velox"
)

const MaxLogSize = int64(10000)

type logMsg struct {
	Pipe string `json:"p"`
	Buff string `json:"b"`
}

type logQueuer struct {
	pipe  string
	queue chan logMsg
}

func (lq *logQueuer) Write(p []byte) (int, error) {
	l := len(p)
	if l > 0 {
		lq.queue <- logMsg{lq.pipe, string(p)}
	}
	return l, nil
}

type agent struct {
	//log
	logQueue chan logMsg
	//proc
	proc       *exec.Cmd
	stdbuff    bytes.Buffer
	expectExit bool
	//http
	root http.Handler
	fs   http.Handler
	sync http.Handler
	//sync
	data struct {
		sync.Mutex
		velox.State
		Config  Config
		Uptime  time.Time
		Files   map[string]string
		LogSize int64
		Log     map[int64]logMsg
	}
}

func newAgent(c Config) *agent {
	a := &agent{}
	a.logQueue = make(chan logMsg)
	a.data.Config = c
	a.data.Uptime = time.Now()
	a.data.Files = map[string]string{}
	a.data.Log = map[int64]logMsg{}
	//http
	router := http.HandlerFunc(a.router)
	if c.User != "" || c.Pass != "" {
		a.root = cookieauth.Wrap(router, c.User, c.Pass)
	} else {
		a.root = router
	}
	if info, err := os.Stat("static/"); err == nil && info.IsDir() {
		a.fs = http.FileServer(http.Dir("static/"))
	} else {
		a.fs = http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, Prefix: "static/"})
	}
	a.sync = velox.SyncHandler(&a.data)
	//fetch files
	go a.readFiles()
	go a.readLog()
	return a
}

func (a *agent) setProc(proc *exec.Cmd) {
	proc.Env = os.Environ()
	proc.Stdout = io.MultiWriter(os.Stdout, &logQueuer{"out", a.logQueue})
	proc.Stderr = io.MultiWriter(os.Stderr, &logQueuer{"err", a.logQueue})
	proc.Stdin = os.Stdin
	a.proc = proc
}

func (a *agent) router(w http.ResponseWriter, r *http.Request) {
	switch filepath.Base(r.URL.Path) {
	case "velox.js":
		velox.JS.ServeHTTP(w, r)
	case "sync":
		a.sync.ServeHTTP(w, r)
	case "configure":
		a.configure(w, r)
	default:
		//fallback to static files
		a.fs.ServeHTTP(w, r)
	}
}

func (a *agent) configure(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", 506)
}

func (a *agent) readFiles() {
	a.data.Lock()
	for _, path := range a.data.Config.ConfigurationFiles {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			a.logf("failed to read file: %s", path)
			continue
		}
		a.data.Files[path] = string(b)
	}
	a.data.Unlock()
	a.data.Push()
}

func (a *agent) readLog() {
	for l := range a.logQueue {
		a.data.Lock()
		a.data.Log[a.data.LogSize] = l
		a.data.LogSize++
		if a.data.LogSize >= MaxLogSize {
			delete(a.data.Log, MaxLogSize-a.data.LogSize)
		}
		a.data.Unlock()
		a.data.Push()
	}
}

func (a *agent) logf(format string, args ...interface{}) {
	log.Printf("[webproc] "+format, args...)
}

func (a *agent) debugf(format string, args ...interface{}) {
	log.Printf("[webproc] "+format, args...)
}
