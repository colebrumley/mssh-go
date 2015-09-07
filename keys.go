package main

import (
	"errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
)

// Getting keys:
//   > --key flag overrides all
//   > If no --key is defined, check for ssh-agent on $SSH_AUTH_SOCK
//   > If no ssh-agent defined, look or key in ~/.ssh/id_rsa
//   > fail if none of the above work
func getKeyAuths(keyfile ...string) (auths []ssh.AuthMethod, err error) {
	auths = []ssh.AuthMethod{}
	if len(keyfile) == 0 {
		// If no keys are provided see if there's an ssh-agent running
		if len(os.Getenv("SSH_AUTH_SOCK")) > 0 {
			if auths, err = loadEnvAgent(); err != nil {
				return
			}
		} else {
			// Otherwise use ~/.ssh/id_rsa or ~/ssh/id_rsa (for windows, but
			// it works on linux too)
			if auths, err = loadDefaultKeys(); err != nil {
				return
			}
		}
	} else {
		// Append each provided key to auths
		auths, err = parseKeyFiles(keyfile)
	}
	if len(auths) == 0 {
		err = errors.New("No auths parsed from provided keys")
	}
	return
}

func parseKeyFiles(paths []string) (auths []ssh.AuthMethod, err error) {
	for _, key := range paths {
		var (
			pemBytes []byte
			signer   ssh.Signer
		)
		if !fileExists(key) {
			err = errors.New("Specified key does not exist")
			return
		}
		pemBytes, err = ioutil.ReadFile(key)
		if err != nil {
			return
		}
		signer, err = ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	return
}

func loadEnvAgent() (auths []ssh.AuthMethod, err error) {
	sshAuthSock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return
	}
	defer sshAuthSock.Close()
	ag := agent.NewClient(sshAuthSock)
	auths = []ssh.AuthMethod{ssh.PublicKeysCallback(ag.Signers)}
	return
}

func loadDefaultKeys() (auths []ssh.AuthMethod, err error) {
	k := ""
	currentUser, err := user.Current()
	defaultKeyPathA := filepath.FromSlash(currentUser.HomeDir + "/.ssh/id_rsa")
	defaultKeyPathB := filepath.FromSlash(currentUser.HomeDir + "/ssh/id_rsa")
	if fileExists(defaultKeyPathA) {
		k = defaultKeyPathA
	} else if fileExists(defaultKeyPathB) {
		k = defaultKeyPathB
	}
	if len(k) == 0 {
		err = errors.New("No key specified")
		return
	}
	pemBytes, err := ioutil.ReadFile(k)
	if err != nil {
		return
	}
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return
	}
	auths = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	return
}
