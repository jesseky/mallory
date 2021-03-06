package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
)

// ConfigFile Memory representation for mallory.json
type ConfigFile struct {
	// private file file
	PrivateKey string `json:"id_rsa"`
	// local addr to listen and serve, default is 127.0.0.1:1315
	LocalSmartServer string `json:"local_smart"`
	// local addr to listen and serve, default is 127.0.0.1:1316
	LocalNormalServer string `json:"local_normal"`
	// remote addr to connect, e.g. ssh://user@linode.my:22
	RemoteServer string `json:"remote"`
	// direct to proxy dial timeout
	SSHDialTimeoutSecond int `json:"ssh_dial_timeout_second"`
	// blocked host list
	BlockedList []string `json:"blocked"`
}

// NewConfigFile Load file from path
func NewConfigFile(path string) (self *ConfigFile, err error) {
	self = &ConfigFile{
		LocalSmartServer:  "127.0.0.1:1315",
		LocalNormalServer: "127.0.0.1:1316",
	}
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(buf, self)
	if err != nil {
		return
	}
	self.PrivateKey = os.ExpandEnv(self.PrivateKey)
	sort.Strings(self.BlockedList)
	return
}

// Blocked test whether host is in blocked list or not
func (self *ConfigFile) Blocked(host string) bool {
	i := sort.SearchStrings(self.BlockedList, host)
	return i < len(self.BlockedList) && self.BlockedList[i] == host
}

// Config Provide global config for mallory
type Config struct {
	// file path
	Path string
	// config file content
	File *ConfigFile
	// mutex for config file
	sync.RWMutex
	loaded bool
}

// NewConfig
func NewConfig(path string) (self *Config, err error) {

	self = &Config{
		Path: os.ExpandEnv(path),
	}
	err = self.Load()
	return
}

func (self *Config) Reload() (err error) {
	file, err := NewConfigFile(self.Path)
	if err != nil {
		L.Printf("Reload %s failed: %s\n", self.Path, err)
	} else {
		L.Printf("Reload %s\n", self.Path)
		self.Lock()
		self.File = file
		self.Unlock()
	}
	return
}

// reload config file
func (self *Config) Load() (err error) {
	if self.loaded {
		panic("can not be reload manually")
	}
	self.loaded = true

	// first time to load
	L.Printf("Loading: %s\n", self.Path)
	self.File, err = NewConfigFile(self.Path)
	if err != nil {
		return
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGHUP)
	go func() {
		for s := range sc {
			if s == syscall.SIGHUP {
				self.Reload()
			}
		}
	}()

	return
}

// test whether host is in blocked list or not
func (self *Config) Blocked(host string) bool {
	self.RLock()
	blocked := self.File.Blocked(host)
	self.RUnlock()
	return blocked
}
