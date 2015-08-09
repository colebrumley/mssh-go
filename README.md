# mssh - multi-ssh client in Go
***Run multiple commands on multiple machines asynchronously***

![shippable tag](https://api.shippable.com/projects/55c6f3c9edd7f2c0529a2f19/badge/master)

`mssh` runs a given list of commands (in order) over ssh to multiple machines.  Each machine get's its own goroutine, so async benefits improve as more servers are added.  Keeping machines in dediated goroutines instead of individual command + host combinations ensures that commands are run in the order listed.

See [mssh](http://sourceforge.net/projects/mssh/) on SourceForge for a more mature Python alternative.

```
NAME:
   mssh - Run SSH commands on multiple machines

USAGE:
   mssh [global options] command [command options] [arguments...]

VERSION:
   0.0.2

COMMANDS:
   help, h	Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --user, -u 						SSH user (defaults to current user) [$MSSH_USER]
   --server, -s [--server option --server option]	Remote server [$MSSH_HOSTS]
   --key, -k 						SSH key (defaults to ~/.ssh/id_rsa) [$MSSH_KEY]
   --fail, -f						Fail immediately if an error is encountered
   --lines, -n "0"					Only show last n lines of output for each cmd
   --color, -c						Print cmd output in color (true by default)
   --help, -h						show help
```

## Example
***Single command, single node***
```
cole ~ $ mssh -u core -s brumley.io "ls -la /srv"
INFO[0000] Result of `ls -la /srv` on core@brumley.io:22
total 12
drwxr-xr-x  3 root root 4096 Aug  5 23:58 .
drwxr-xr-x 17 root root 4096 Aug  5 06:05 ..
drwxr-xr-x  2 1000 root 4096 Aug  7 22:48 minecraftmods

```

***Multiple commands, multiple nodes***
```
cole ~ $ mssh -u core -k ~/.ssh/key.pem -s mothership-01 -s mothership-02 -s mothership-01 "docker images | wc -l" "docker ps | wc -l"
INFO[0001] Result of `docker images | wc -l` on core@mothership-02:22
40
INFO[0001] Result of `docker images | wc -l` on core@mothership-01:22
37
INFO[0001] Result of `docker images | wc -l` on core@mothership-01:22
37
INFO[0001] Result of `docker ps | wc -l` on core@mothership-02:22
13
INFO[0002] Result of `docker ps | wc -l` on core@mothership-01:22
12
INFO[0002] Result of `docker ps | wc -l` on core@mothership-01:22
12
```
