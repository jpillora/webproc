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

  Usage: webproc [options] <arg> [arg] ...

  args can be either a command with arguments or a webproc file

  Options:
  --host, -h                listening interface (default 0.0.0.0)
  --port, -p                listening port (default 8080, env PORT)
  --user, -u                basic auth username (env HTTP_USER)
  --pass                    basic auth password (env HTTP_PASS)
  --allowed-ip, -a          allowed ip or cidr block (allows multiple)
  --log, -l                 log mode (must be 'webui' or 'proxy' or 'both' defaults to 'both')
  --on-exit, -o             process exit action (default ignore)
  --configuration-file, -c  configuration file (allows multiple)
  --restart-timeout, -r     restart timeout controls when to perform a force kill (default 30s)
  --max-lines, -m           maximum number of log lines to show in webui (default 5000)
  --version, -v             display version
  --help                    display help

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
# "ignore" - ignore and wait for manual restart via the web UI
# "proxy" - also exit webproc with the same exit code
# "restart" - automatically restart with exponential backoff time delay between failed restarts
OnExit = "ignore"

# Configuration files to be editable by the web UI.
# For example, dnsmasq would include "/etc/dnsmasq.conf"
ConfigurationFiles = []

# After the restart signal (SIGINT) has been sent, webproc will wait for RestartTimeout before
# forcibly restarting the process (SIGKILL).
RestartTimeout = "30s"
```


#### Contributing

Install Go and setup your GOPATH

``` sh
# setup go, get go-bindata and webproc
$ go get -u github.com/jteeuwen/go-bindata/...
$ go get -u -v github.com/jpillora/webproc
$ cd $GOPATH/src/github.com/jpillora/webproc
# ... edit source ...
$ go generate ./...
$ go install -v
$ $GOPATH/bin/webproc
```

#### MIT License

Copyright © 2017 Jaime Pillora &lt;dev@jpillora.com&gt;

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
'Software'), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED 'AS IS', WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.