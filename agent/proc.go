package agent

import (
	"io"
	"os"
	"os/exec"
	"os/signal"
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

type procRequest struct {
	req    string
	signal os.Signal
	reply  chan bool
}

func (a *agent) runProc(c Config) {
	//lookup path
	progPath, err := exec.LookPath(c.ProgramArgs[0])
	if err != nil {
		a.log.Fatalf("program not found in path: %s", err)
	}
	var proc *exec.Cmd
	var running chan error
	//proc requests
	expectingRestart := false
	awaitingManualRestart := false
	manualRestart := make(chan bool)
	go func() {
		for req := range a.procReqs {
			switch req.req {
			case "start":
				if !awaitingManualRestart {
					a.log.Printf("cant start")
					continue
				}
				manualRestart <- true
			case "restart":
				if !a.running() || running == nil {
					a.log.Printf("cant restart")
					continue
				}
				if err := proc.Process.Signal(a.data.Config.GoRestartSignal); err != nil {
					a.log.Printf("signal failed")
				}
				r := running
				select {
				case <-r:
					//success
					a.log.Printf("restart success")
				case <-time.After(time.Duration(a.data.Config.RestartTimeout)):
					//timeout! kill it!
					if err := a.proc.Process.Kill(); err != nil {
						a.log.Printf("failed to kill process: %s", err)
					} else {
						a.log.Printf("restart timed out")
					}
				}
			default:
				a.log.Printf("unknown request")
			}
		}
	}()
	//proxy signals to proc
	go func() {
		signals := make(chan os.Signal)
		signal.Notify(signals)
		for sig := range signals {
			if proc != nil && proc.Process != nil {
				if err := proc.Process.Signal(sig); err != nil {
					a.log.Printf("failed to signal process: %s", err)
				}
			} else if sig == os.Interrupt {
				a.log.Printf("interupt with no proc, exiting...")
				os.Exit(0)
			} else {
				a.log.Printf("ignored signal: %s", sig)
			}
		}
	}()
	//loop
	b := backoff.Backoff{}
	for {
		//start proc
		expectingRestart = false
		proc = exec.Command(progPath, c.ProgramArgs[1:]...)
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
		a.proc = proc
		a.setRunning(true, proc.Process.Pid)
		//block here while running
		running = make(chan error)
		go func() {
			running <- proc.Wait()
		}()
		err := <-running
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
		proc = nil
		a.proc = nil
		a.setRunning(false, code)
		close(running)
		//exit strategy
		if expectingRestart || c.OnExit == OnExitRestart {
			//delay then restart
			delay := b.Duration()
			a.log.Printf("program exited with %d, restart in %s...", code, delay)
			time.Sleep(delay)
		} else if c.OnExit == OnExitProxy {
			//exit webproc
			a.log.Printf("program exited with %d, exiting webproc...", code)
			time.Sleep(100 * time.Millisecond)
			os.Exit(code)
		} else if c.OnExit == OnExitIgnore {
			//wait for agent to allow restart
			a.log.Printf("program exited with %d, awaiting restart...", code)
			awaitingManualRestart = true
			<-manualRestart
			a.log.Printf("manually restarting...")
		} else {
			panic("unknown onexit")
		}
	}
}
