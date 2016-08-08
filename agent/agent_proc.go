package agent

import (
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jpillora/backoff"
)

type Sig string

func (s Sig) Signal() {
	return
}

func (s Sig) String() string {
	return string(s)
}

var (
	procRunning  = int64(1)
	procExited   = int64(0)
	procChanging = int64(-1)
)

type procRequest struct {
	kind   string
	signal os.Signal
	reply  chan bool
}

func (a *agent) runProc(c Config) {
	//lookup path
	progPath, err := exec.LookPath(c.ProgramArgs[0])
	if err != nil {
		a.log.Fatalf("program not found in path: %s", err)
	}
	//loop
	b := backoff.Backoff{}
	for {
		code := a.runProcOnce(progPath, c)
		if code == 0 {
			a.log.Printf("program exited successfully, restarting...")
			b.Reset()
		} else {
			delay := b.Duration()
			a.log.Printf("program exited with %d, restart in %s...", code, delay)
			time.Sleep(delay)
		}
	}
}

func (a *agent) runProcOnce(prog string, c Config) int {
	//empty requests queue
	// for len(a.procReqs) > 0 {
	// 	<-a.procReqs
	// }
	//start proc
	proc := exec.Command(prog, c.ProgramArgs[1:]...)
	proc.Env = os.Environ()
	if wd, err := os.Getwd(); err == nil {
		proc.Dir = wd
	}
	proc.Stdout = io.MultiWriter(os.Stdout, &msgQueuer{"out", a.msgQueue})
	proc.Stderr = io.MultiWriter(os.Stderr, &msgQueuer{"err", a.msgQueue})
	proc.Stdin = os.Stdin
	if err := proc.Start(); err != nil {
		a.log.Fatalf("program failed to start: %s", err)
	}
	running := make(chan struct{})
	manualRestart := int64(0)
	//convert proc.wait into goroutine
	wait := make(chan int)
	go func() {
		err := proc.Wait()
		close(running)
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
		//stop accepting signals
		a.setRunning(false, code)
		//
		wait <- code
	}()
	//start accepting requests
	a.setRunning(true, proc.Process.Pid)
	//block here while running
	for {
		select {
		case req := <-a.procReqs:
			switch req {
			case "restart":
				a.log.Printf("manually restarting...")
				go func() {
					atomic.StoreInt64(&manualRestart, 1)
					a.procSigs <- os.Interrupt
					a.log.Printf("manual restart req sent")
				}()
			default:
				a.log.Printf("ignoring request: %s", req)
			}
		case sig := <-a.procSigs:
			//interupt? start kill timer
			if sig == os.Interrupt {
				go func() {
					select {
					case <-running:
						//success
						a.log.Printf("restart success")
					case <-time.After(time.Duration(a.data.Config.RestartTimeout)):
						//timeout! kill it!
						if err := proc.Process.Kill(); err != nil {
							a.log.Printf("failed to kill process: %s", err)
						} else {
							a.log.Printf("force killed process")
						}
					}
				}()
			}
			//issue signal
			a.log.Printf("sending signal: %s", sig)
			time.Sleep(100 * time.Millisecond)
			proc.Process.Signal(sig)
		case code := <-wait:
			//manually restarted?
			if atomic.LoadInt64(&manualRestart) == 1 {
				return code
			}
			//exit strategy
			switch c.OnExit {
			case OnExitRestart:
				return code
			case OnExitProxy:
				//exit webproc
				a.log.Printf("program exited with %d, exiting webproc...", code)
				time.Sleep(100 * time.Millisecond)
				os.Exit(code)
			case OnExitIgnore:
				//wait for agent to allow restart
				a.log.Printf("program exited with %d, awaiting restart...", code)
				for req := range a.procReqs {
					switch req {
					case "restart":
						a.log.Printf("manually starting...")
						return code
					default:
						a.log.Printf("ignoring request: %s", req)
					}
				}
			}
		}
	}
}
