package main

import (
	"fmt"
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
	"runtime"
	"strings"
)

func main() {
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
			Name:  "color",
			Usage: "Print cmd output in color (use --color=false to disable)",
		},
	}
	app.Action = defaultAction
	app.Run(os.Args)
}

func defaultAction(c *cli.Context) {
	hosts := c.StringSlice("server")

	currentUser, err := user.Current()
	if err != nil {
		print(fmt.Sprintf("[X] Could not get current user: %v", err), "")
	}
	u := c.String("user")
	// If no flag, get current username
	if len(u) == 0 {
		u = currentUser.Username
		if len(u) == 0 {
			print("[X] No username specified!", "")
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
			print("[*] Attempting to use existing ssh-agent", "")
			conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
			if err != nil {
				return
			}
			defer conn.Close()
			ag := agent.NewClient(conn)
			auths = []ssh.AuthMethod{ssh.PublicKeysCallback(ag.Signers)}
		} else {
			k := ""
			// Otherwise use ~/.ssh/id_rsa or ~/ssh/id_rsa (for windows)
			if fileExists(currentUser.HomeDir + string(filepath.Separator) + ".ssh" + string(filepath.Separator) + "id_rsa") {
				k = currentUser.HomeDir + string(filepath.Separator) + ".ssh" + string(filepath.Separator) + "id_rsa"
			} else if fileExists(currentUser.HomeDir + string(filepath.Separator) + "ssh" + string(filepath.Separator) + "id_rsa") {
				k = currentUser.HomeDir + string(filepath.Separator) + "ssh" + string(filepath.Separator) + "id_rsa"
			}
			if len(k) == 0 {
				print("[X] No key specified: "+err.Error(), "")
				cli.ShowAppHelp(c)
				os.Exit(2)
			}
			pemBytes, err := ioutil.ReadFile(k)
			if err != nil {
				print("[X] Error reading key: "+err.Error(), "")
				os.Exit(2)
			}
			signer, err := ssh.ParsePrivateKey(pemBytes)
			if err != nil {
				print("[X] Error reading key: "+err.Error(), "")
				os.Exit(2)
			}
			auths = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		}
	} else {
		if !fileExists(key) {
			print("[X] Specified key does not exist!", "")
			os.Exit(1)
		}
		pemBytes, err := ioutil.ReadFile(key)
		if err != nil {
			print("[X] "+err.Error(), "")
		}
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			print("[X] "+err.Error(), "")
		}
		auths = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}

	// host(s) is required
	if len(hosts) == 0 {
		print("At least one host is required", "yellow")
		cli.ShowAppHelp(c)
		os.Exit(2)
	}

	// At least one command is required
	if len(c.Args().First()) == 0 {
		print("At least one command is required", "yellow")
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
					col := ""
					print(fmt.Sprintf("[X] Execution of `%s` on %s@%s failed. Error message: %v", cmd, u, host, err), "")
					if c.Bool("color") {
						col = "red"
					}
					print(out, col)
					if c.Bool("fail") {
						os.Exit(1)
					}
				} else {
					print(fmt.Sprintf("[*] Execution of `%s` on %s@%s succeeded:", cmd, u, host), "")
					col := ""
					if c.Bool("color") {
						col = "green"
					}
					print(out, col)
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

func print(str string, c string) {
	if len(c) == 0 || runtime.GOOS == "windows" {
		fmt.Printf("%s\n", str)
		return
	}
	switch c {
	case "red":
		fmt.Printf("%s\n", color.RedString("%s", str))
	case "green":
		fmt.Printf("%s\n", color.GreenString("%s", str))
	case "yellow":
		fmt.Printf("%s\n", color.YellowString("%s", str))
	default:
		fmt.Printf("%s\n", str)
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
