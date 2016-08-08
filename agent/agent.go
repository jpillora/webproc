//go:generate go-bindata -pkg agent -ignore .../.DS_Store -o agent_static.go -prefix static/ static/...

package agent

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
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
	msgQueue chan msg
	//proc
	procState int64
	procReqs  chan string
	procSigs  chan os.Signal
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
	a.procState = procChanging
	a.procReqs = make(chan string)
	a.procSigs = make(chan os.Signal)
	//sync state
	a.data.Config = c
	a.data.Running = false
	a.data.Manual = c.OnExit == OnExitIgnore
	a.data.Files = map[string]string{}
	a.data.Log = map[int64]msg{}
	a.data.LogOffset = 0
	a.data.LogMaxSize = 10000
	a.sync = velox.SyncHandler(&a.data)
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
	//grab listener
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		return fmt.Errorf("failed to start server: %s", err)
	}
	//threads
	go a.readLog()
	go a.runProc(c)
	//load from disk
	a.readFiles()
	//catch all signals
	go func() {
		signals := make(chan os.Signal)
		signal.Notify(signals)
		for sig := range signals {
			if a.running() {
				//proxy through to proc
				a.procSigs <- sig
			} else if sig == os.Interrupt {
				a.log.Printf("interupt with no proc, exiting...")
				os.Exit(0)
			} else {
				a.log.Printf("ignored signal: %s", sig)
			}
		}
	}()
	//serve agent's root handler
	a.log.Printf("agent listening on http://%s:%d...", c.Host, c.Port)
	return http.Serve(l, a)
}

func (a *agent) setRunning(running bool, value int) {
	a.data.Lock()
	a.data.Running = running
	a.data.ChangedAt = time.Now()
	if running {
		atomic.StoreInt64(&a.procState, procRunning)
		a.data.Pid = value
		a.data.ExitCode = 0
	} else {
		atomic.StoreInt64(&a.procState, procExited)
		a.data.Pid = 0
		a.data.ExitCode = value
	}
	a.data.Unlock()
	a.data.Push()
}

func (a *agent) running() bool {
	return atomic.LoadInt64(&a.procState) == procRunning
}

func (a *agent) restart() {
	a.procReqs <- "restart"
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
