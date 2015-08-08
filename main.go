package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"os"
	"os/user"
	"strings"
)

func main() {
	app := cli.NewApp()
	app.Name = "mssh"
	app.Usage = "Run SSH commands on multiple machines"
	app.Version = "0.0.1"
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
	}
	app.Action = defaultAction
	app.Run(os.Args)
}

func defaultAction(c *cli.Context) {
	hosts := c.StringSlice("server")
	u := c.String("user")
	if len(u) == 0 {
		// get current user if unset
		usr, err := user.LookupId(fmt.Sprintf("%d", os.Getuid()))
		if err != nil {
			log.Fatalf("Could not look up current user: %v", err)
		}
		u = usr.Username
	}

	key := c.String("key")
	if len(key) == 0 {
		// If no key provided use ~/.ssh/id_rsa
		key = os.Getenv("HOME") + "/.ssh/id_rsa"
	}

	// user and host(s) required
	if len(hosts) == 0 {
		log.Fatalln("At least one host is required")
	}
	argList := append([]string{c.Args().First()}, c.Args().Tail()...)
	chLen := len(hosts) * len(argList)
	done := make(chan bool, chLen)
	for _, h := range hosts {
		// Hosts are handled asynchronously
		go func(host string, cmds []string) {
			// Set the default port if none specified
			if len(strings.Split(host, ":")) == 1 {
				host = host + ":22"
			}
			for _, cmd := range cmds {
				combined, err := RunRemoteCmd(u, host, key, cmd)
				if err != nil {
					log.Fatalf("FAILED: %v", err)
				}
				log.Printf("Result of `%s` on %s@%s\n%s", cmd, u, host, combined)
				done <- true
			}
		}(h, argList)
	}

	// Drain chan before exiting
	for i := 0; i < chLen; i++ {
		<-done
	}
}
