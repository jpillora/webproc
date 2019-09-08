//go:generate statik -src static

package agent

import (
	"compress/gzip"
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

	"github.com/NYTimes/gziphandler"
	"github.com/jpillora/cookieauth"
	"github.com/jpillora/ipfilter"
	"github.com/jpillora/requestlog"
	"github.com/jpillora/velox"
	"github.com/rakyll/statik/fs"

	//embed static assets
	_ "github.com/jpillora/webproc/agent/statik"
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
		Version       string
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

func Run(version string, c Config) error {
	a := &agent{}
	a.msgQueue = make(chan msg, 10000)
	agentWriter := &msgQueuer{"agent", a.msgQueue}
	a.log = log.New(io.MultiWriter(os.Stdout, agentWriter), "[webproc] ", log.LstdFlags)
	a.procState = procChanging
	a.procReqs = make(chan string)
	a.procSigs = make(chan os.Signal)
	//sync state
	a.data.State.Throttle = 250 * time.Millisecond
	a.data.Version = version
	a.data.Config = c
	a.data.Running = false
	a.data.Manual = c.OnExit == OnExitIgnore
	a.data.Files = map[string]string{}
	a.data.Log = map[int64]msg{}
	a.data.LogOffset = 0
	a.data.LogMaxSize = int64(c.MaxLines)
	a.sync = velox.SyncHandler(&a.data)
	//http
	h := http.Handler(http.HandlerFunc(a.router))
	//custom middleware stack
	//4. gzip
	gzipper, _ := gziphandler.NewGzipLevelAndMinSize(
		gzip.DefaultCompression, 0)
	h = gzipper(h)
	//3. basic-auth middleware
	if c.User != "" || c.Pass != "" {
		h = cookieauth.Wrap(h, c.User, c.Pass)
	}
	//2. ipfilter middlware
	if len(c.AllowedIPs) > 0 || len(c.AllowedCountries) > 0 {
		if len(c.AllowedIPs) == 0 {
			a.log.Printf("auto-allow localhost (127.0.0.1)")
			c.AllowedIPs = append(c.AllowedIPs, "127.0.0.1")
		}
		h = ipfilter.Wrap(h, ipfilter.Options{
			AllowedIPs:       c.AllowedIPs,
			AllowedCountries: c.AllowedCountries,
			TrustProxy:       c.TrustProxy,
			BlockByDefault:   true,
			Logger:           a.log,
		})
	}
	//1. log middleware (log everything!)
	var reqlogs io.Writer
	if c.Log == LogWebUI {
		reqlogs = agentWriter
	} else {
		io.MultiWriter(os.Stdout, agentWriter)
	}
	h = requestlog.WrapWith(h, requestlog.Options{
		Writer: reqlogs,
		Colors: &requestlog.Colors{},
		Format: `[webproc] {{ if .Timestamp }}{{ .Timestamp }} {{end}}` +
			`{{ .Method }} {{ .Path }} {{ .Code }} ` +
			`{{ .Duration }}{{ if .Size }} {{ .Size }}{{end}}` +
			`{{ if .IP }} ({{ .IP }}){{end}}` + "\n",
	})
	a.root = h
	//filesystem
	if info, err := os.Stat("agent/static/"); err == nil && info.IsDir() {
		a.log.Printf("agent serving local static files")
		a.fs = http.FileServer(http.Dir("agent/static/"))
	} else {
		statikFS, err := fs.New()
		if err != nil {
			return fmt.Errorf("failed to load static assets: %s", err)
		}
		a.fs = http.FileServer(statikFS)
	}
	//grab listener
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", c.Host, c.Port))
	if err != nil {
		return fmt.Errorf("failed to start server: %s", err)
	}
	//threads
	go a.runProc(c)
	go a.readLog()
	//load from disk
	a.readFiles()
	//catch all signals
	go func() {
		signals := make(chan os.Signal)
		signal.Notify(signals)
		for sig := range signals {
			if sig == os.Interrupt {
				a.log.Printf("webproc interupted, exiting...")
				if a.running() {
					a.procSigs <- os.Kill
					time.Sleep(100 * time.Millisecond)
				}
				os.Exit(0)
			} else if a.running() {
				//proxy through to proc
				a.procSigs <- sig
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
	a.data.Unlock()
	if changed {
		a.log.Printf("loaded config files changes from disk")
		a.data.Push()
	}
}

func (a *agent) readLog() {
	for l := range a.msgQueue {
		a.data.Lock()
		o := a.data.LogOffset
		a.data.Log[o] = l
		if o >= a.data.LogMaxSize {
			delete(a.data.Log, o-a.data.LogMaxSize)
		}
		a.data.LogOffset++
		a.data.Unlock()
		a.data.Push()
	}
}
