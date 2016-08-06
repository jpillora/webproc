//go:generate go-bindata -pkg wp -ignore .../.DS_Store -o static.go -prefix static/ static/...

package wp

import (
	"bytes"
	"encoding/json"
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
	"github.com/jpillora/requestlog"
	"github.com/jpillora/velox"
)

type Agent struct {
	//log
	log           *log.Logger
	verb          *log.Logger
	msgQueue      chan msg
	manualRestart chan bool
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
		Config     Config
		Uptime     time.Time
		Running    bool
		Manual     bool
		Pid        int
		Files      map[string]string
		LogOffset  int64
		LogMaxSize int64
		Log        map[int64]msg
	}
}

func NewAgent(c Config) *Agent {
	a := &Agent{}
	a.msgQueue = make(chan msg)
	if c.OnExit == OnExitIgnore {
		a.manualRestart = make(chan bool)
	}
	agentWriter := &msgQueuer{"agent", a.msgQueue}
	a.log = log.New(io.MultiWriter(os.Stdout, agentWriter), "[webproc] ", log.LstdFlags)
	a.verb = log.New(agentWriter, "[webproc] ", log.LstdFlags)
	//sync state
	a.data.Config = c
	a.data.Running = false
	a.data.Manual = a.manualRestart != nil
	a.data.Files = map[string]string{}
	a.data.Log = map[int64]msg{}
	a.data.LogOffset = 0
	a.data.LogMaxSize = 10000
	//http
	h := http.Handler(http.HandlerFunc(a.router))
	//custom middleware
	if c.User != "" || c.Pass != "" {
		log.Printf("cookieauth")
		h = cookieauth.Wrap(h, c.User, c.Pass)
	}
	if true {
		h = requestlog.WrapWith(h, requestlog.Options{
			Writer: agentWriter,
			Colors: &requestlog.Colors{},
			Format: `[webproc] {{ if .Timestamp }}{{ .Timestamp }} {{end}}` +
				`{{ .Method }} {{ .Path }} {{ .Code }} ` +
				`{{ .Duration }}{{ if .Size }} {{ .Size }}{{end}}` +
				`{{ if .IP }} ({{ .IP }}){{end}}` + "\n",
		})
	}
	a.root = h
	//filesystem
	if info, err := os.Stat("wp/static/"); err == nil && info.IsDir() {
		a.fs = http.FileServer(http.Dir("wp/static/"))
	} else {
		a.fs = http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo})
	}
	a.sync = velox.SyncHandler(&a.data)
	//fetch files
	go a.readFiles()
	go a.readLog()
	//
	a.log.Printf("agent listening on http://%s:%d...", c.Host, c.Port)
	return a
}

func (a *Agent) setProc(proc *exec.Cmd) {
	if proc != nil {
		//started?
		a.expectExit = false
		proc.Stdout = io.MultiWriter(os.Stdout, &msgQueuer{"out", a.msgQueue})
		proc.Stderr = io.MultiWriter(os.Stderr, &msgQueuer{"err", a.msgQueue})
		proc.Stdin = os.Stdin
		a.proc = proc
	} else {
		//exited?
		a.proc = nil
	}
	a.checkProc()
}

func (a *Agent) checkProc() {
	a.data.Lock()
	a.data.Running = a.proc != nil
	if a.proc != nil && a.proc.Process != nil {
		a.data.Pid = a.proc.Process.Pid
		if a.data.Uptime.IsZero() {
			a.data.Uptime = time.Now()
		}
	} else {
		a.data.Pid = 0
		a.data.Uptime = time.Time{}
	}
	a.data.Unlock()
	a.data.Push()
}

func (a *Agent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.root.ServeHTTP(w, r)
}

func (a *Agent) router(w http.ResponseWriter, r *http.Request) {
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

func (a *Agent) configure(w http.ResponseWriter, r *http.Request) {
	data := struct {
		File     string
		Contents string
	}{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "configure json error", 400)
		return
	}
	//ensure in whitelist
	allowed := false
	for _, f := range a.data.Config.ConfigurationFiles {
		if f == data.File {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "invalid file", 400)
		return
	}
	perms := os.FileMode(600)
	//use existing perms if able
	exists := false
	if info, err := os.Stat(data.File); err == nil {
		perms = info.Mode().Perm()
		exists = true
	}
	f, err := os.OpenFile(data.File, os.O_RDWR, perms)
	if err != nil {
		http.Error(w, "failed to open file", 500)
		return
	}
	var newb = []byte(data.Contents)
	var b []byte
	if exists {
		b, err = ioutil.ReadAll(f)
		if err != nil {
			http.Error(w, "failed to read file", 500)
			return
		}
		if bytes.Equal(b, newb) {
			http.Error(w, "no change", 400)
			return
		}
		f.Seek(0, 0)
		f.Truncate(0)
	}
	if _, err := f.Write(newb); err != nil {
		http.Error(w, "failed to write changes", 500)
		return
	}
	a.readFiles()
	//if not running, just write file
	if !a.data.Running {
		if a.manualRestart != nil {
			a.manualRestart <- true
		}

		w.WriteHeader(200)
		return
	}
	a.expectExit = true
	if err := a.proc.Process.Signal(a.data.Config.GoRestartSignal); err != nil {
		log.Printf("%s", err)
		http.Error(w, "failed to signal process", 500)
		return
	}
}

func (a *Agent) readFiles() {
	a.data.Lock()
	for i, path := range a.data.Config.ConfigurationFiles {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			a.log.Printf("failed to read configuration file (#%d): %s", i, path)
			continue
		}
		a.data.Files[path] = string(b)
	}
	a.data.Unlock()
	a.data.Push()
}

func (a *Agent) readLog() {
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
