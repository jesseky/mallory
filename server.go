// Package mallory implements a simple http proxy support direct and GAE remote fetcher
package main

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

const (
	SmartSrv = iota
	NormalSrv
)

type AccessType bool

func (t AccessType) String() string {
	if t {
		return "PROXY"
	} else {
		return "DIRECT"
	}
}

type Server struct {
	// SmartSrv or NormalSrv
	Mode int
	// config file
	Cfg *Config
	// direct fetcher
	Direct *Direct
	// ssh fetcher, to connect remote proxy server
	SSH *SSH
	// a cache
	BlockedHosts map[string]bool
	// for serve http
	sync.RWMutex
}

// Create and intialize
func NewServer(mode int, c *Config) (self *Server, err error) {
	ssh, err := NewSSH(c)
	if err != nil {
		return
	}

	shouldProxyTimeout := time.Millisecond * time.Duration(c.File.ShouldProxyTimeoutMS)

	self = &Server{
		Mode:         mode,
		Cfg:          c,
		Direct:       NewDirect(shouldProxyTimeout),
		SSH:          ssh,
		BlockedHosts: make(map[string]bool),
	}
	return
}

func (self *Server) Blocked(host string) bool {
	blocked, cached := false, false
	host = HostOnly(host)
	self.RLock()
	if self.BlockedHosts[host] {
		blocked = true
		cached = true
	}
	self.RUnlock()

	if !blocked {
		tld, _ := publicsuffix.EffectiveTLDPlusOne(host)
		blocked = self.Cfg.Blocked(tld)
	}

	if !blocked {
		suffix, _ := publicsuffix.PublicSuffix(host)
		blocked = self.Cfg.Blocked(suffix)
	}

	if blocked && !cached {
		self.Lock()
		self.BlockedHosts[host] = true
		self.Unlock()
	}
	return blocked
}

// HTTP proxy accepts requests with following two types:
//  - CONNECT
//    Generally, this method is used when the client want to connect server with HTTPS.
//    In fact, the client can do anything he want in this CONNECT way...
//    The request is something like:
//      CONNECT www.google.com:443 HTTP/1.1
//    Only has the host and port information, and the proxy should not do anything with
//    the underlying data. What the proxy can do is just exchange data between client and server.
//    After accepting this, the proxy should response
//      HTTP/1.1 200 OK
//    to the client if the connection to the remote server is established.
//    Then client and server start to exchange data...
//
//  - non-CONNECT, such as GET, POST, ...
//    In this case, the proxy should redo the method to the remote server.
//    All of these methods should have the absolute URL that contains the host information.
//    A GET request looks like:
//      GET weibo.com/justmao945/.... HTTP/1.1
//    which is different from the normal http request:
//      GET /justmao945/... HTTP/1.1
//    Because we can be sure that all of them are http request, we can only redo the request
//    to the remote server and copy the reponse to client.
//
func (self *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	use := (self.Blocked(r.URL.Host) || self.Mode == NormalSrv) && r.URL.Host != ""
	L.Printf("[%s] %s %s %s\n", AccessType(use), r.Method, r.RequestURI, r.Proto)

	if r.Method == "CONNECT" {
		if use {
			self.SSH.Connect(w, r)
		} else {
			err := self.Direct.Connect(w, r)
			if err == ErrShouldProxy {
				self.SSH.Connect(w, r)
			}
		}
	} else if r.URL.IsAbs() {
		// This is an error if is not empty on Client
		r.RequestURI = ""
		RemoveHopHeaders(r.Header)
		if use {
			self.SSH.ServeHTTP(w, r)
		} else {
			err := self.Direct.ServeHTTP(w, r)
			if err == ErrShouldProxy {
				self.SSH.ServeHTTP(w, r)
			}
		}
	} else if r.URL.Path == "/reload" {
		self.reload(w, r)
	} else {
		L.Printf("%s is not a full URL path\n", r.RequestURI)
	}
}

func (self *Server) reload(w http.ResponseWriter, r *http.Request) {
	err := self.Cfg.Reload()
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(self.Cfg.Path + ": " + err.Error()))
	} else {
		w.WriteHeader(200)
		w.Write([]byte(self.Cfg.Path + " reloaded"))
	}
}
