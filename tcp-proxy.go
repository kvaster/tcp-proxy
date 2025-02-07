package main

import (
	"flag"
	"github.com/apex/log"
	"github.com/kvaster/apexutils"
	"os"
	"os/signal"
	"syscall"
	"tcp-proxy/proxy"
)

var listenAddr = flag.String("listen.addr", ":8883", "listen address and port")
var mark = flag.Int("mark", 0, "mark flow")

func main() {
	flag.Parse()
	apexutils.ParseFlags()

	log.Info("starting tcp-proxy")

	s := proxy.New(*listenAddr, *mark)

	if err := s.Start(); err != nil {
		log.WithError(err).Error("error starting tcp-proxy")
		os.Exit(1)
	}

	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	<-stopChan

	log.Info("stopping tcp-proxy")
	s.Stop()

	log.Info("stopped tcp-proxy")
}
