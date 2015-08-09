package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/fatih/color"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io/ioutil"
	"net"
	"os"
	// Use of the os/user package prevents cross-compilation
	"os/user" // <- https://github.com/golang/go/issues/6376
	"path/filepath"
	"strings"
)

func main() {
	app := cli.NewApp()
	app.HideVersion = true
	app.Name = "mssh"
	app.Usage = "Run SSH commands on multiple machines"
	app.Version = "0.0.2.1"
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
			Usage: "Print cmd output in color (use -c=false to disable)",
		},
	}
	app.Action = defaultAction
	app.Run(os.Args)
}

func defaultAction(c *cli.Context) {
	hosts := c.StringSlice("server")

	currentUser, err := user.Current()
	if err != nil {
		log.Errorf("Could not get current user: %v", err)
	}
	u := c.String("user")
	// If no flag, get current username
	if len(u) == 0 {
		u = currentUser.Username
		if len(u) == 0 {
			log.Errorln("No username specified!")
		}
	}

	// Getting keys:
	// --key flag overrides all
	// If no --key is defined, check for ssh-agent on $SSH_AUTH_SOCK
	// If no ssh-agent defined, look or key in ~/.ssh/id_rsa
	// fail if none of the above work
	key := c.String("key")
	auths := []ssh.AuthMethod{}
	if len(key) == 0 {
		// If no key provided see if there's an ssh-agent running
		if len(os.Getenv("SSH_AUTH_SOCK")) > 0 {
			log.Println("Attempting to use existing ssh-agent")
			conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
			if err != nil {
				return
			}
			defer conn.Close()
			ag := agent.NewClient(conn)
			auths = []ssh.AuthMethod{ssh.PublicKeysCallback(ag.Signers)}
		} else {
			// Otherwise use ~/.ssh/id_rsa
			key = filepath.FromSlash(currentUser.HomeDir + "/.ssh/id_rsa")
			if !fileExists(key) {
				log.Errorln("Must specify a key, ~/.ssh/id_rsa does not exist and no ssh-agent is available!")
				cli.ShowAppHelp(c)
				os.Exit(2)
			}
			pemBytes, err := ioutil.ReadFile(key)
			if err != nil {
				log.Errorf("%v", err)
			}
			signer, err := ssh.ParsePrivateKey(pemBytes)
			if err != nil {
				log.Errorf("%v", err)
			}
			auths = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		}
	} else {
		if !fileExists(key) {
			log.Errorln("Specified key does not exist!")
			os.Exit(1)
		}
		pemBytes, err := ioutil.ReadFile(key)
		if err != nil {
			log.Errorf("%v", err)
		}
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			log.Errorf("%v", err)
		}
		auths = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}

	// host(s) is required
	if len(hosts) == 0 {
		log.Warnln("At least one host is required")
		cli.ShowAppHelp(c)
		os.Exit(2)
	}

	// At least one command is required
	if len(c.Args().First()) == 0 {
		log.Warnln("At least one command is required")
		cli.ShowAppHelp(c)
		os.Exit(2)
	}

	// append list of commands to argList
	argList := append([]string{c.Args().First()}, c.Args().Tail()...)
	chLen := len(hosts) * len(argList)
	done := make(chan bool, chLen)
	for _, h := range hosts {
		// Hosts are handled asynchronously
		go func(c *cli.Context, host string, cmds []string, auth []ssh.AuthMethod) {
			// Set the default port if none specified
			if len(strings.Split(host, ":")) == 1 {
				host = host + ":22"
			}
			for _, cmd := range cmds {
				combined, err := runRemoteCmd(u, host, auth, cmd)
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
		}(c, h, argList, auths)
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
