package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mesos/mesos-go/detector"
	"github.com/mesosphere/mesos-dns/detect"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/resolver"
	"github.com/mesosphere/mesos-dns/util"
)

func main() {
	util.PanicHandlers = append(util.PanicHandlers, func(_ interface{}) {
		// by default the handler already logs the panic
		os.Exit(1)
	})

	var versionFlag bool

	// parse flags
	cjson := flag.String("config", "config.json", "path to config file (json)")
	flag.BoolVar(&versionFlag, "version", false, "output the version")
	flag.Parse()

	// -version
	if versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	// initialize logging
	logging.SetupLogs()

	// initialize resolver
	config := records.SetConfig(*cjson)
	res := resolver.New(version, config)
	errch := make(chan error)

	// launch DNS server
	if config.DNSOn {
		go func() { errch <- <-res.LaunchDNS() }()
	}

	// launch HTTP server
	if config.HTTPOn {
		go func() { errch <- <-res.LaunchHTTP() }()
	}

	changed := make(chan []string, 1)
	connected := make(chan bool, 1)

	go func() {
		if config.Zk != "" {
			logging.Verbose.Println("Starting master detector for ZK ", config.Zk)
			if md, err := detector.New(config.Zk); err != nil {
				log.Fatalf("failed to create master detector: %v", err)
			} else if err := md.Detect(detect.NewMasters(config.Masters, changed)); err != nil {
				log.Fatalf("failed to initialize master detector: %v", err)
			}
		} else {
			changed <- config.Masters
			connected <- true
		}
	}()

	reload := time.NewTicker(time.Second * time.Duration(config.RefreshSeconds))

	zkTimeout := time.Second * time.Duration(config.ZkDetectionTimeout)
	timeout := time.AfterFunc(zkTimeout, func() {
		connected <- false
	})

	res.Reload()
	defer reload.Stop()
	defer util.HandleCrash()
	for {
		select {
		case <-reload.C:
			res.Reload()
		case masters := <-changed:
			timeout.Stop()
			logging.VeryVerbose.Printf("new masters detected: %v", masters)
			res.SetMasters(masters)
			res.Reload()
		case err := <-errch:
			logging.VeryVerbose.Println(err)
		case isConnected := <-connected:
			if isConnected {
				logging.Verbose.Println("Connected to masters")
			} else {
				logging.VeryVerbose.Println("Not yet connected to masters")
			}
		}
	}
}
