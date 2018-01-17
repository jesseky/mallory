package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/publicsuffix"
)

var (
	// global logger
	L       = log.New(os.Stdout, "mallory: ", log.Lshortfile|log.LstdFlags)
	FConfig = flag.String("config", "$HOME/.config/mallory.json", "config file")
	FSuffix = flag.String("suffix", "", "print pulbic suffix for the given domain")
	FReload = flag.Bool("reload", false, "send signal to reload config file")
)

var (
	// forward
	Forword  = flag.Bool("forwardmode", false, "forward mode") // forwardmode
	FNetwork = flag.String("network", "tcp", "network protocol")
	FListen  = flag.String("listen", ":20022", "listen on this port")
	FForward = flag.String("forward", ":80", "destination address and port")
)

func forward() {
	L = log.New(os.Stdout, "forward: ", log.Lshortfile|log.LstdFlags)

	L.Printf("Listening on %s for %s...\n", *FListen, *FNetwork)
	ln, err := net.Listen(*FNetwork, *FListen)
	if err != nil {
		L.Fatal(err)
	}

	for id := 0; ; id++ {
		conn, err := ln.Accept()
		if err != nil {
			L.Printf("%d: %s\n", id, err)
			continue
		}
		L.Printf("%d: new %s\n", id, conn.RemoteAddr())

		if tcpConn := conn.(*net.TCPConn); tcpConn != nil {
			L.Printf("%d: setup keepalive for TCP connection\n", id)
			tcpConn.SetKeepAlive(true)
			tcpConn.SetKeepAlivePeriod(30 * time.Second)
		}

		go func(myid int, conn net.Conn) {
			defer conn.Close()
			c, err := net.Dial(*FNetwork, *FForward)
			if err != nil {
				L.Printf("%d: %s\n", myid, err)
				return
			}
			L.Printf("%d: new %s <-> %s\n", myid, c.RemoteAddr(), conn.RemoteAddr())
			defer c.Close()
			wait := make(chan int)
			go func() {
				n, err := io.Copy(c, conn)
				if err != nil {
					L.Printf("%d: %s\n", myid, err)
				}
				L.Printf("%d: %s -> %s %d bytes\n", myid, conn.RemoteAddr(), c.RemoteAddr(), n)
				close(wait)
			}()
			go func() {
				n, err := io.Copy(conn, c)
				if err != nil {
					L.Printf("%d: %s\n", myid, err)
				}
				L.Printf("%d: %s -> %s %d bytes\n", myid, c.RemoteAddr(), conn.RemoteAddr(), n)
				close(wait)
			}()
			<-wait
			L.Printf("%d: connection closed\n", myid)
		}(id, conn)
	}
}

func serve() {
	L.Printf("Starting...\n")
	L.Printf("PID: %d\n", os.Getpid())

	c, err := NewConfig(*FConfig)
	if err != nil {
		L.Fatalln(err)
	}

	L.Printf("Connecting remote SSH server: %s\n", c.File.RemoteServer)

	wait := make(chan int)
	go func() {
		normal, err := NewServer(NormalSrv, c)
		if err != nil {
			L.Fatalln(err)
		}
		L.Printf("Local normal HTTP proxy: %s\n", c.File.LocalNormalServer)
		L.Fatalln(http.ListenAndServe(c.File.LocalNormalServer, normal))
		wait <- 1
	}()

	go func() {
		smart, err := NewServer(SmartSrv, c)
		if err != nil {
			L.Fatalln(err)
		}
		L.Printf("Local smart HTTP proxy: %s\n", c.File.LocalSmartServer)
		L.Fatalln(http.ListenAndServe(c.File.LocalSmartServer, smart))
		wait <- 1
	}()
	<-wait
}

func printSuffix() {
	host := *FSuffix
	tld, _ := publicsuffix.EffectiveTLDPlusOne(host)
	fmt.Printf("EffectiveTLDPlusOne: %s\n", tld)
	suffix, _ := publicsuffix.PublicSuffix(host)
	fmt.Printf("PublicSuffix: %s\n", suffix)
}

func reload() {
	file, err := NewConfigFile(os.ExpandEnv(*FConfig))
	if err != nil {
		L.Fatal(err)
	}
	res, err := http.Get(fmt.Sprintf("http://%s/reload", file.LocalNormalServer))
	if err != nil {
		L.Fatal(err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		L.Fatal(err)
	}
	fmt.Printf("%s\n", body)
}

func main() {
	flag.Parse()
	if *Forword { // forward mode
		forward()
		return
	}
	if *FSuffix != "" {
		printSuffix()
	} else if *FReload {
		reload()
	} else {
		serve()
	}
}
