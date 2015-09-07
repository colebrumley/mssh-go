package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/fatih/color"
	"golang.org/x/crypto/ssh"
	"os"
	// Use of the os/user package prevents cross-compilation
	"os/user" // <- https://github.com/golang/go/issues/6376
	"runtime"
	"strings"
)

type AppCfg struct {
	Hosts []*HostConfig
	Cmds  []string
	Lines int
	Color bool
	Fail  bool
}

type HostConfig struct {
	User  string
	Addr  string
	Auths []ssh.AuthMethod
}

type StdOutLogger struct {
	Color     bool
	Time      bool
	okPrefix  string
	errPrefix string
}

func (s *StdOutLogger) Success(message string, content string) {
	var str string
	if runtime.GOOS == "windows" {
		str = fmt.Sprintf("%s %s", s.okPrefix, message)
	} else {
		str = fmt.Sprintf("%s", color.GreenString("%s %s", s.okPrefix, message))
	}
	fmt.Printf("%s\n%s\n", str, content)
}

func (s *StdOutLogger) Error(message string, err error) {
	var str string
	if runtime.GOOS == "windows" {
		str = fmt.Sprintf("%s %s Error: %v", s.errPrefix, message, err)
	} else {
		str = fmt.Sprintf("%s", color.RedString("%s %s Error: %v", s.errPrefix, message, err))
	}
	fmt.Printf("%s\n", str)
}

func NewStdOutLogger() *StdOutLogger {
	return &StdOutLogger{
		okPrefix:  "[*]",
		errPrefix: "[X]",
	}
}

var (
	log *StdOutLogger
)

func main() {
	log = NewStdOutLogger()
	app := cli.NewApp()
	app.HideVersion = true
	app.Name = "mssh"
	app.Usage = "Run SSH commands on multiple machines"
	app.Version = "0.0.3"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "user,u",
			Usage:  "SSH user (defaults to current user)",
			EnvVar: "MSSH_USER",
		},
		cli.StringSliceFlag{
			Name:   "server,s",
			Usage:  "Remote server",
			EnvVar: "MSSH_SERVERS",
		},
		cli.StringSliceFlag{
			Name:   "key,k",
			Usage:  "SSH key (defaults to ~/.ssh/id_rsa)",
			EnvVar: "MSSH_KEYS",
		},
		cli.BoolFlag{
			Name:  "fail,f",
			Usage: "Fail immediately if an error is encountered",
		},
		cli.IntFlag{
			Name:  "lines,n",
			Usage: "Only show last n lines of output for each cmd",
		},
		cli.BoolTFlag{
			Name:  "color,colour,c",
			Usage: "Print color output (--color=false to disable)",
		},
	}
	app.Action = defaultAction
	app.Run(os.Args)
}

func defaultAction(c *cli.Context) {
	thisCfg := new(AppCfg)
	thisCfg.Lines = c.Int("lines")
	thisCfg.Fail = c.Bool("fail")
	thisCfg.Color = c.BoolT("color")

	// If no flag, get current username
	u := c.String("user")
	if len(u) == 0 {
		usr, err := user.Current()
		if err == nil {
			u = usr.Username
		}
	}

	// Load SSH keys
	auths, err := getKeyAuths(c.GlobalStringSlice("key")...)
	if err != nil {
		log.Error("Error loading key", err)
	}

	if len(auths) == 0 {
		log.Error("No keys defined, cannot continue!", nil)
		cli.ShowAppHelp(c)
		os.Exit(2)
	}

	for _, target := range c.StringSlice("server") {
		thisHost := &HostConfig{
			User:  u,
			Auths: auths,
			Addr:  portAddrCheck(target),
		}
		thisCfg.Hosts = append(thisCfg.Hosts, thisHost)
	}

	// host(s) is required
	if len(thisCfg.Hosts) == 0 {
		log.Error("At least one host is required", nil)
		cli.ShowAppHelp(c)
		os.Exit(2)
	}

	// At least one command is required
	if len(c.Args().First()) == 0 {
		log.Error("At least one command is required", nil)
		cli.ShowAppHelp(c)
		os.Exit(2)
	}
	// append list of commands to argList
	thisCfg.Cmds = append([]string{c.Args().First()}, c.Args().Tail()...)
	done := make(chan bool, len(thisCfg.Hosts))

	for _, h := range thisCfg.Hosts {
		go handleHost(h, thisCfg.Cmds, thisCfg.Color, thisCfg.Lines, thisCfg.Fail, done)
	}

	// Drain chan before exiting
	for i := 0; i < len(thisCfg.Hosts); i++ {
		<-done
	}
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	// The last line is always empty
	lines = lines[:len(lines)-1]

	// Only trim if n > 0
	if n <= 0 {
		return strings.Join(lines, "\n")
	}

	// Return the original string if it has
	// fewer lines than n
	if len(lines) < n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-(n):], "\n")
}

func handleHost(host *HostConfig, cmds []string, color bool, lines int, fail bool, done chan bool) {
	for _, cmd := range cmds {
		combined, err := runRemoteCmd(host.User, host.Addr, host.Auths, cmd)
		out := tail(string(combined), lines)
		if err != nil {
			log.Error(fmt.Sprintf("Execution of `%s` on %s@%s failed", cmd, host.User, host.Addr), err)
			if fail {
				done <- true
				return
			}
		} else {
			log.Success(fmt.Sprintf("Execution of `%s` on %s@%s succeeded:", cmd, host.User, host.Addr), out)
		}
	}
	done <- true
}

func runRemoteCmd(user string, addr string, auth []ssh.AuthMethod, cmd string) (out []byte, err error) {
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User: user,
		Auth: auth,
	})
	if err != nil {
		return
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return
	}
	defer session.Close()
	out, err = session.CombinedOutput(cmd)
	return
}

// fileExists returns a bool if os.Stat returns an IsNotExist error
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func portAddrCheck(addr string) string {
	if len(strings.Split(addr, ":")) == 1 {
		return addr + ":22"
	}
	return addr
}
