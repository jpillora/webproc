//go:generate go-bindata -pkg agent -ignore .../.DS_Store -o static.go -prefix static/ static/...

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/jpillora/cookieauth"
	"github.com/jpillora/ipfilter"
	"github.com/jpillora/requestlog"
	"github.com/jpillora/velox"
)

type agent struct {
	//log
	log      *log.Logger
	verb     *log.Logger
	msgQueue chan msg
	//proc
	proc     *exec.Cmd
	procReqs chan procRequest
	//http
	root http.Handler
	fs   http.Handler
	sync http.Handler
	//sync
	data struct {
		sync.Mutex
		velox.State
		Config        Config
		ChangedAt     time.Time
		Running       bool
		Manual        bool
		Pid, ExitCode int
		Files         map[string]string
		LogOffset     int64
		LogMaxSize    int64
		Log           map[int64]msg
	}
}

func Run(c Config) error {
	a := &agent{}
	a.msgQueue = make(chan msg)
	agentWriter := &msgQueuer{"agent", a.msgQueue}
	a.log = log.New(io.MultiWriter(os.Stdout, agentWriter), "[webproc] ", log.LstdFlags)
	a.verb = log.New(agentWriter, "[webproc] ", log.LstdFlags)
	a.procReqs = make(chan procRequest)
	//sync state
	a.data.Config = c
	a.data.Running = false
	a.data.Manual = c.OnExit == OnExitIgnore
	a.data.Files = map[string]string{}
	a.data.Log = map[int64]msg{}
	a.data.LogOffset = 0
	a.data.LogMaxSize = 10000
	//http
	h := http.Handler(http.HandlerFunc(a.router))
	//custom middleware stack
	//3. basic-auth middleware
	if c.User != "" || c.Pass != "" {
		log.Printf("cookieauth")
		h = cookieauth.Wrap(h, c.User, c.Pass)
	}
	//2. ipfilter middlware
	if len(c.AllowedIPs) > 0 {
		h = ipfilter.Wrap(h, ipfilter.Options{
			AllowedIPs:     c.AllowedIPs,
			BlockByDefault: true,
		})
	}
	//1. log middleware (log everything!)
	h = requestlog.WrapWith(h, requestlog.Options{
		Writer: agentWriter,
		Colors: &requestlog.Colors{},
		Format: `[webproc] {{ if .Timestamp }}{{ .Timestamp }} {{end}}` +
			`{{ .Method }} {{ .Path }} {{ .Code }} ` +
			`{{ .Duration }}{{ if .Size }} {{ .Size }}{{end}}` +
			`{{ if .IP }} ({{ .IP }}){{end}}` + "\n",
	})
	a.root = h
	//filesystem
	if info, err := os.Stat("agent/static/"); err == nil && info.IsDir() {
		a.fs = http.FileServer(http.Dir("agent/static/"))
	} else {
		a.fs = http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo})
	}
	a.sync = velox.SyncHandler(&a.data)
	//grab listener
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		return fmt.Errorf("failed to start server: %s", err)
	}
	//threads
	go a.readLog()
	go a.runProc(c)
	a.readFiles()
	//serve agent's root handler
	a.log.Printf("agent listening on http://%s:%d...", c.Host, c.Port)
	return http.Serve(l, a)
}

func (a *agent) setRunning(running bool, value int) {
	a.data.Lock()
	a.data.Running = running
	a.data.ChangedAt = time.Now()
	if running {
		a.data.Pid = value
		a.data.ExitCode = 0
	} else {
		a.data.Pid = 0
		a.data.ExitCode = value
	}
	a.data.Unlock()
	a.data.Push()
}

func (a *agent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.root.ServeHTTP(w, r)
}

func (a *agent) router(w http.ResponseWriter, r *http.Request) {
	switch filepath.Base(r.URL.Path) {
	case "velox.js":
		velox.JS.ServeHTTP(w, r)
	case "sync":
		a.sync.ServeHTTP(w, r)
	case "start":
		a.start(w, r)
	case "save":
		a.save(w, r)
	default:
		//fallback to static files
		a.fs.ServeHTTP(w, r)
	}
}

func (a *agent) running() bool {
	return a.proc != nil && a.proc.Process != nil
}

func (a *agent) start(w http.ResponseWriter, r *http.Request) {
	if !a.running() {
		a.procReqs <- procRequest{
			req: "start",
		}
		a.log.Printf("triggered manual start")
		w.WriteHeader(200)
		return
	}
	//user restart
	if err := a.restart(); err != nil {
		http.Error(w, "failed to restart", 500)
		return
	}
	w.WriteHeader(200)
}

func (a *agent) save(w http.ResponseWriter, r *http.Request) {
	files := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&files); err != nil {
		http.Error(w, "json error", 400)
		return
	}
	if len(files) == 0 {
		http.Error(w, "no files", 400)
		return
	}
	//ensure in whitelist
	for f, _ := range files {
		allowed := false
		for _, configFile := range a.data.Config.ConfigurationFiles {
			if f == configFile {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "invalid file", 400)
			return
		}
	}
	for f, contents := range files {
		perms := os.FileMode(600)
		//use existing perms if able
		exists := false
		if info, err := os.Stat(f); err == nil {
			perms = info.Mode().Perm()
			exists = true
		}
		var newb = []byte(contents)
		if exists {
			b, err := ioutil.ReadFile(f)
			if err != nil {
				http.Error(w, "failed to read file", 500)
				return
			}
			if bytes.Equal(b, newb) {
				http.Error(w, "no change", 400)
				return
			}
		}
		if err := ioutil.WriteFile(f, newb, perms); err != nil {
			http.Error(w, "failed to write changes", 500)
			return
		}
	}
	if a.running() {
		if err := a.restart(); err != nil {
			http.Error(w, "failed to restart", 500)
			return
		}
	}
	a.readFiles()
	w.WriteHeader(200)
	return
}

func (a *agent) restart() error {
	a.procReqs <- procRequest{
		req:    "restart",
		signal: a.data.Config.GoRestartSignal,
	}
	return nil
}

func (a *agent) readFiles() {
	a.data.Lock()
	changed := false
	for i, path := range a.data.Config.ConfigurationFiles {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			a.log.Printf("failed to read configuration file (#%d): %s", i, path)
			continue
		}
		existing := a.data.Files[path]
		curr := string(b)
		if curr != existing {
			a.data.Files[path] = curr
			changed = true
		}
	}
	if changed {
		a.log.Printf("loaded config files changes from disk")
	}
	a.data.Unlock()
	a.data.Push()
}

func (a *agent) readLog() {
	for l := range a.msgQueue {
		a.data.Lock()
		a.data.Log[a.data.LogOffset] = l
		a.data.LogOffset++
		if a.data.LogOffset >= a.data.LogMaxSize {
			delete(a.data.Log, a.data.LogMaxSize-a.data.LogOffset)
		}
		a.data.Unlock()
		a.data.Push()
	}
}
