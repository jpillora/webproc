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
	LogWebUI     = "webui"
	LogProxy     = "proxy"

	OnExitRestart OnExit = "restart"
	OnExitIgnore         = "ignore"
	OnExitProxy          = "proxy"
)

//Config is shared for both toml unmarshalling and opts CLI generation
type Config struct {
	Host               string   `help:"listening interface"`
	Port               int      `help:"listening port" env:"PORT"`
	User               string   `help:"basic auth username" env:"HTTP_USER"`
	Pass               string   `help:"basic auth password" env:"HTTP_PASS"`
	AllowedIPs         []string `opts:"-"`
	ProgramArgs        []string `type:"arglist" min:"1" name:"args" help:"args can be either a command with arguments or a webproc file"`
	Log                Log      `opts:"-"`
	OnExit             OnExit   `help:"process exit action" default:"ignore"`
	ConfigurationFiles []string `name:"config" type:"commalist" help:"comma-separated list of configuration files"`
	RestartTimeout     Duration `opts:"-"`
	MaxLines           int      `help:"maximum number of log lines to show in webui" default:"5000"`
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
