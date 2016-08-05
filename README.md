
:warning: In progress

---

# webproc

Wrap any program in a simple web-based user-interface

## Install

...

## Usage

Let's use `webproc` to run `dnsmasq`:

```
webproc dnsmasq.toml
```

Where `dnsmasq.toml` is:

``` toml
Command = "dnsmasq"
```

Now, you can visit http://localhost:8080 and see the process logs and status

## Config

Here is a complete configuration with the current defaults:

[embedmd]:# (default.toml)
```toml
# Interface to serve web UI. Warning: defaults to ALL interfaces.
Host = "0.0.0.0"

# Port to serve web UI
Port = 8080

# Basic authentication settings for web UI
User = ""
Pass = ""

# Command to execute (with optional arguments). Note: the process
# must remain in the foreground (i.e. do NOT fork/run as daemon).
Command = ""

# Log settings for the process:
# "both" - log to both, webproc standard out/error and to the web UI log.
# "webui" - only log to the web UI. Note, the web UI only keeps the last 10k lines.
# "proxy" - only log to webproc standard out/error.
Log = "both"

# OnExit dictates what action to take when the process exits:
# "proxy" - also exit webproc with the same exit code
# "ignore" - ignore and wait for manual restart via the web UI
# "restart" - automatically restart with exponential backoff time delay between failed restarts
# It is recommended to use "proxy" and then to run webproc commands via a process manager.
OnExit = "proxy"

# Configuration files to be editable by the web UI.
# For example, dnsmasq would include "/etc/dnsmasq.conf"
ConfigurationFiles = []

# When provided, this command will be used verify all configuration changes
# before restarting the process. An exit code 0 means valid, otherwise it's assumed invalid.
VerifyCommand = ""

# When provided, this signal is used to restart the process. It's set to interrupt (SIGINT) by default, though
# some programs support zero down-time configuration reloads via SIGHUP, SIGUSR2, etc.
RestartSignal = "SIGINT"

# After the restart signal has been sent, webproc will wait for RestartTimeout before
# forcibly restarting the process.
RestartTimeout = "30s"
```
