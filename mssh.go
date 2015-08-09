package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/fatih/color"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	app := cli.NewApp()
	app.HideVersion = true
	app.Name = "mssh"
	app.Usage = "Run SSH commands on multiple machines"
	app.Version = "0.0.2"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "user,u",
			Usage:  "SSH user (defaults to current user)",
			EnvVar: "MSSH_USER",
		},
		cli.StringSliceFlag{
			Name:   "server,s",
			Usage:  "Remote server",
			EnvVar: "MSSH_HOSTS",
		},
		cli.StringFlag{
			Name:   "key,k",
			Usage:  "SSH key (defaults to ~/.ssh/id_rsa)",
			EnvVar: "MSSH_KEY",
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
			Name:  "color,c",
			Usage: "Print cmd output in color (true by default)",
		},
	}
	app.Action = defaultAction
	app.Run(os.Args)
}

func defaultAction(c *cli.Context) {
	hosts := c.StringSlice("server")
	u := c.String("user")

	key := c.String("key")
	if len(key) == 0 {
		// If no key provided use ~/.ssh/id_rsa
		key = os.Getenv("HOME") + "/.ssh/id_rsa"
		if !fileExists(key) {
			log.Errorln("Must specify a key, ~/.ssh/id_rsa does not exist!")
			cli.ShowAppHelp(c)
			os.Exit(2)
		}
	}

	// user and host(s) required
	if len(hosts) == 0 {
		log.Errorln("At least one host is required")
		cli.ShowAppHelp(c)
		os.Exit(2)
	}
	if len(c.Args().First()) == 0 {
		cli.ShowAppHelp(c)
		os.Exit(2)
	}
	argList := append([]string{c.Args().First()}, c.Args().Tail()...)
	chLen := len(hosts) * len(argList)
	done := make(chan bool, chLen)
	for _, h := range hosts {
		// Hosts are handled asynchronously
		go func(c *cli.Context, host string, cmds []string) {
			// Set the default port if none specified
			if len(strings.Split(host, ":")) == 1 {
				host = host + ":22"
			}
			for _, cmd := range cmds {
				combined, err := runRemoteCmd(u, host, key, cmd)
				out := tail(string(combined), c.Int("lines"))
				if err != nil {
					pretty := prettyOutput(
						fmt.Sprintf("Execution of `%s` on %s@%s failed. Error message: %v", cmd, u, host, err),
						out, true, c.Bool("color"))
					if c.Bool("fail") {
						log.Fatalln(pretty)
					}
					log.Errorf(pretty)
				} else {
					pretty := prettyOutput(
						fmt.Sprintf("Execution of `%s` on %s@%s succeeded:", cmd, u, host),
						out, false, c.Bool("color"))
					log.Println(pretty)
				}
				done <- true
			}
		}(c, h, argList)
	}

	// Drain chan before exiting
	for i := 0; i < chLen; i++ {
		<-done
	}
}

func prettyOutput(status string, out string, isErr bool, useColor bool) (formatted string) {

	formatted = status
	if useColor {
		if isErr {
			formatted = fmt.Sprintf("%s\n%s", formatted, color.RedString("%s", out))
		} else {
			formatted = fmt.Sprintf("%s\n%s", formatted, color.GreenString("%s", out))
		}
		return
	}
	if isErr {
		formatted = fmt.Sprintf("%s\n%s", formatted, out)
	} else {
		formatted = fmt.Sprintf("%s\n%s", formatted, out)
	}
	return
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

func runRemoteCmd(user string, addr string, key string, cmd string) (out []byte, err error) {
	pemBytes, err := ioutil.ReadFile(key)
	if err != nil {
		return
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return
	}
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
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

// FileExists returns a bool if os.Stat returns an IsNotExist error
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
