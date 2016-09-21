# webproc

Wrap any program in a simple web-based user-interface

## Install

**Binaries**

See [the latest release](https://github.com/jpillora/webproc/releases/latest) or download it now with `curl https://i.jpillora.com/webproc | bash`

**Source**

``` sh
$ go get -v github.com/jpillora/webproc
```

## Usage

Let's use `webproc` to run `dnsmasq`:

```
webproc --config /etc/dnsmasq.conf -- dnsmasq --no-daemon
```

Visit [http://localhost:8080](http://localhost:8080) and view the process configuration, status and logs.

<img width="747" alt="screen shot 2016-09-22 at 1 39 01 am" src="https://cloud.githubusercontent.com/assets/633843/18718069/7d515392-8065-11e6-8ba5-86b6e59f3992.png">

For more features, see the [Configuration](#Configuration) file

## CLI

```
$ webproc --help

    Usage: webproc [options] args...

    args can be either a command with arguments or a webproc file

    Options:
    --host, -h     listening interface
    --port, -p     listening port (env PORT)
    --user, -u     basic auth username (env USER)
    --pass         basic auth password (env PASS)
    --on-exit, -o  process exit action (default proxy)
    --config, -c   comma-separated list of configuration files
    --help
    --version, -v

    Version:
      0.0.0-src

    Read more:
      https://github.com/jpillora/webproc

```

## Configuration

The CLI interface only exposes a subset of the configuration, to further customize
webproc, create a `program.toml` file and then load it with:

```
webproc program.toml
```

Here is a complete configuration with the defaults, only `ProgramArgs` is **required**:

[embedmd]:# (default.toml)
```toml
# Program to execute (with optional Arguments). Note: the process
# must remain in the foreground (i.e. do NOT fork/run as daemon).
ProgramArgs = []

# Interface to serve web UI. Warning: defaults to ALL interfaces.
Host = "0.0.0.0"

# Port to serve web UI
Port = 8080

# Basic authentication settings for web UI
User = ""
Pass = ""

# IP addresses which should be allowed to access the web UI
# For example, ["10.0.0.0/8"]
AllowedIPs = []

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

# After the restart signal (SIGINT) has been sent, webproc will wait for RestartTimeout before
# forcibly restarting the process (SIGKILL).
RestartTimeout = "30s"
```
