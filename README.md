# mssh - multi-ssh client in Go
***Run multiple commands on multiple machines asynchronously***

![shippable tag](https://api.shippable.com/projects/55c6f3c9edd7f2c0529a2f19/badge/master)

`mssh` is a small, cross-platform, multi-threaded SSH client designed to run a series of commands in order across any number of hosts.  It's entirely key-based, and does not support non-PKI authentication.

This was a weekend project for me. While it's a fairly simple little app, there aren't any tests beyond my functional testing (*PRs welcome :)*).  I wouldn't rely on `mssh` for highly risky operations.

*See [mssh](http://sourceforge.net/projects/mssh/) for a Python alternative.*

```
NAME:
   mssh - Run SSH commands on multiple machines

USAGE:
   mssh [global options] command [command options] [arguments...]

VERSION:
   0.0.3

COMMANDS:
   help, h	Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --user, -u 						SSH user (defaults to current user) [$MSSH_USER]
   --server, -s [--server option --server option]	Remote server [$MSSH_HOSTS]
   --key, -k 						SSH key (defaults to ~/.ssh/id_rsa) [$MSSH_KEY]
   --fail, -f						Fail immediately if an error is encountered
   --lines, -n "0"					Only show last n lines of output for each cmd
   --color						Print cmd output in color (use --color=false to disable)
   --help, -h						show help
```
## Examples
```sh
# Get the number of running Docker containers
# and % used of /dev/xvdb
mssh \
  --user core \
  --server dev-01.lab.dev \
  --server dev-02.lab.dev \
  --server dev-03.lab.dev \
  'docker ps -q | wc -l' \
  'df -h | grep /dev/xvdb | awk "{print \$5}"'
  ```
## Usage
#### `ssh-agent` Support
`mssh` does not provide its own ssh-agent server, but it will fall back to an existing agent specified by `SSH_AUTH_SOCK` if no explicit `--key` is defined.

#### Detached shells
`mssh` runs each command in a detached shell.  This means that there is no interactivity, so things like answering prompts are not possible. IO redirection and piping within a command work though, most interactivity needs can be accomodated that way.

The combined stderr and stdout streams are printed on completion.  You can grab the last n lines with the `--lines` flag, similar to `tail`.

#### Servers
There are 2 ways to specify target servers: via the `--server` flag, or the `$MSSH_HOSTS` environment variable.  Using both flags and the environment variable is allowed (i.e. flags do not override the env).  The lists are combined but not deduplicated, that's a todo item.  Ports can be specified by appending `:<number>` to the server name.

**Env:**
```sh
# comma separated list of hosts
export MSSH_HOSTS=test1.dev.lab,test2.dev.lab,test3.dev.lab:2222
```

**Flag:**
```sh
# Ports can be specified by appending ":<number>"
mssh --server test4.dev.lab --server 10.0.99.9:2022 ...
```

#### Platform Support
I've tested `mssh` primarily on OSX and 64 bit Linux.  The changes for v0.0.3 were to accomodate the Windows command prompt, but I've only lightly tested functionality.  `mssh` does not display color on Windows prompts, and commands need to be double-quoted (not single).

The default RSA key paths are `~/.ssh/id_rsa` or `~/ssh/id_rsa` on Linux and OSX, and `%PROFILEDIR%\ssh\id_rsa` on Windows.

