## mallory
HTTP/HTTPS proxy over SSH.


## Installation
* Local machine: `go get github.com/jesseky/mallory`
* Remote server: need our old friend sshd


## Configueration
### Config file
Default path is `$HOME/.config/mallory.json`, can be set when start program
```
mallory -config path/to/config.json
```

Content:
* `id_rsa` is the path to our private key file, can be generated by `ssh-keygen`
* `local_smart` is the local address to serve HTTP proxy with smart detection of destination host
* `local_normal` is similar to `local_smart` but send all traffic through remote SSH server without destination host detection
* `remote` is the remote address of SSH server
* `blocked` is a list of domains that need use proxy, any other domains will connect to their server directly

```json
{
  "id_rsa": "$HOME/.ssh/id_rsa",
  "local_smart": ":1315",
  "local_normal": ":1316",
  "remote": "ssh://user@vm.me:22",
  "blocked": [
    "angularjs.org",
    "golang.org",
    "google.com",
    "google.co.jp",
    "googleapis.com",
    "googleusercontent.com",
    "google-analytics.com",
    "gstatic.com",
    "twitter.com",
    "youtube.com"
  ]
}
```

Blocked list in config file will be reloaded automatically when updated, and you can do it manually:
```
# send signal to reload
kill -USR2 <pid of mallory>

# or use reload command by sending http request
mallory -reload
```

### System config
* Set both HTTP and HTTPS proxy to `localhost` with port `1315` to use with block list
* Set env var `http_proxy` and `https_proxy` to `localhost:1316` for terminal usage

### Get the right suffix name for a domain
```
mallory -suffix www.google.com
```

### A simple command to forward all traffic for the given port
```sh
# install it: I merged forward command to mallory, use mallory -forwardmode to start it 

# all traffic through port 20022 will be forwarded to destination.com:22
mallory -forwardmode -network tcp -listen :20022 -forward destination.com:22

# you can ssh to destination:22 through localhost:20022
ssh root@localhost -p 20022
```

### TODO
* return http error when unable to dial
* add host to list automatically when unable to dial
* support multiple remote servers
