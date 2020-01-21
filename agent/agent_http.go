package agent

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jpillora/velox"
)

func (a *agent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.root.ServeHTTP(w, r)
}

func (a *agent) router(w http.ResponseWriter, r *http.Request) {
	switch filepath.Base(r.URL.Path) {
	case "velox.js":
		velox.JS.ServeHTTP(w, r)
	case "sync":
		a.sync.ServeHTTP(w, r)
	case "restart":
		a.serveRestart(w, r)
	case "refresh":
		a.serveRefresh(w, r)
	case "save":
		a.serveSave(w, r)
	default:
		//fallback to static files
		a.fs.ServeHTTP(w, r)
	}
}

func (a *agent) serveRestart(w http.ResponseWriter, r *http.Request) {
	a.restart() //user restart
	w.WriteHeader(200)
}

func (a *agent) serveRefresh(w http.ResponseWriter, r *http.Request) {
	a.readFiles() //user refresh config files
	w.WriteHeader(200)
}

func (a *agent) serveSave(w http.ResponseWriter, r *http.Request) {
	files := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&files); err != nil {
		http.Error(w, "json error", 400)
		return
	}
	if len(files) == 0 {
		http.Error(w, "no files", 400)
		return
	}
	//ensure in file whitelist
	for f := range files {
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

	a.readFiles()
	time.Sleep(100 * time.Millisecond)
	//a.restart()
	w.WriteHeader(200)
	return
}
