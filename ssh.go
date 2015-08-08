package main

import (
	"bytes"
	log "github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
)

func RunRemoteCmd(user string, addr string, key string, cmd string) (out string, err error) {
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
		log.Errorf("session failed:%v", err)
		return
	}
	defer session.Close()
	var outBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &outBuf
	err = session.Run(cmd)
	out = string(outBuf.Bytes())
	if err != nil {
		log.Errorf("Run failed:%v\n%s", err, out)
	}
	return
}
