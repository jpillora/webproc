package agent

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/naoina/toml"
)

type (
	Log      string
	OnExit   string
	Duration time.Duration
)

const (
	LogBoth  Log = "both"
	LogWebUI Log = "webui"
	LogProxy Log = "proxy"

	OnExitRestart OnExit = "restart"
	OnExitIgnore  OnExit = "ignore"
	OnExitProxy   OnExit = "proxy"
)

//Config is shared for both toml unmarshalling and opts CLI generation.
//Defaults are applied on ValidateConfig.
type Config struct {
	Host               string   `opts:"help=listening interface, default=0.0.0.0"`
	Port               int      `opts:"help=listening port, default=8080, env=PORT"`
	User               string   `opts:"help=basic auth username, env=HTTP_USER"`
	Pass               string   `opts:"help=basic auth password, env=HTTP_PASS"`
	AllowedIPs         []string `opts:"name=allow-ip, help=allow ip or cidr block"`
	AllowedCountries   []string `opts:"name=allow-country, short=y, help=allow ip range by 2-letter ISO country code"`
	TrustProxy         bool     `opts:"help=trust proxy HTTP headers to provide remote ip address"`
	ProgramArgs        []string `opts:"mode=arg, name=arg, help=args can be either a command with arguments or a webproc file, min=1"`
	Log                Log      `opts:"help=log mode (must be 'webui' or 'proxy' or 'both' defaults to 'both')"`
	OnExit             OnExit   `opts:"help=process exit action, default=ignore"`
	ConfigurationFiles []string `opts:"mode=flag, help=writable configuration file"`
	RestartTimeout     Duration `opts:"help=restart timeout controls when to perform a force kill, default=30s"`
	MaxLines           int      `opts:"help=maximum number of log lines to show in webui, default=5000"`
}

func LoadConfig(path string, c *Config) error {
	if info, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("file not found")
	} else if err != nil {
		return fmt.Errorf("file error: %s", err)
	} else if info.IsDir() {
		return fmt.Errorf("file not specified")
	}
	cbuff, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("file read error: %s", err)
	}
	//toml reader
	if err := toml.Unmarshal(cbuff, c); err != nil {
		return fmt.Errorf("toml syntax error: %s", err)
	}
	return nil
}

func ValidateConfig(c *Config) error {
	if len(c.ProgramArgs) == 0 {
		return fmt.Errorf("required property ProgramArgs is missing")
	}
	//apply defaults
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
	if c.Port == 0 {
		c.Port = 8080
	}
	if c.MaxLines == 0 {
		c.MaxLines = 5000
	}
	switch c.Log {
	case LogBoth, LogProxy, LogWebUI:
	default:
		c.Log = LogBoth
	}
	switch c.OnExit {
	case OnExitProxy, OnExitIgnore, OnExitRestart:
	default:
		c.OnExit = OnExitIgnore
	}
	if c.RestartTimeout <= 0 {
		c.RestartTimeout = Duration(30 * time.Second)
	}
	return nil
}

// helper types

func (o *OnExit) UnmarshalTOML(data []byte) error {
	*o = OnExit(quoted(data))
	return nil
}

func (o *OnExit) Set(s string) error {
	*o = OnExit(s)
	return nil
}

func (o *OnExit) String() string {
	return string(*o)
}

func (o *Log) UnmarshalTOML(data []byte) error {
	*o = Log(quoted(data))
	return nil
}

func (d *Duration) UnmarshalTOML(data []byte) error {
	str := quoted(data)
	d2, err := time.ParseDuration(str)
	*d = Duration(d2)
	return err
}

func quoted(data []byte) string {
	if l := len(data); l >= 2 {
		return string(data[1 : l-1])
	}
	return string(data)
}
