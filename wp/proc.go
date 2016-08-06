package wp

import (
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

func RunProcess(c Config, a *Agent) {
	//lookup path
	progPath, err := exec.LookPath(c.ProgramArgs[0])
	if err != nil {
		a.log.Fatalf("program not found in path: %s", err)
	}
	var proc *exec.Cmd
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
		proc = exec.Command(progPath, c.ProgramArgs[1:]...)
		proc.Env = os.Environ()
		if wd, err := os.Getwd(); err == nil {
			proc.Dir = wd
		}
		a.setProc(proc)
		if err := proc.Start(); err != nil {
			a.log.Fatalf("program failed to start: %s", err)
		}
		a.checkProc()
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
		a.setProc(nil)
		proc = nil
		//exit strategy
		if a.expectExit || c.OnExit == OnExitRestart {
			//delay then restart
			delay := b.Duration()
			a.verb.Printf("program exited with %d, restart in %s...", code, delay)
			time.Sleep(delay)
		} else if c.OnExit == OnExitProxy {
			//exit webproc
			a.verb.Printf("program exited with %d, exiting webproc...", code)
			os.Exit(code)
		} else if c.OnExit == OnExitIgnore {
			//wait for agent to allow restart
			a.verb.Printf("program exited with %d, awaiting restart...", code)
			<-a.manualRestart
		} else {
			panic("unknown onexit")
		}
	}
}
