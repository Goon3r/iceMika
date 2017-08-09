// Copyright 2015 toor@titansof.tv
//
// A (currently) stateless torrent tracker using Redis as a backing store
//
// Performance tuning options for linux kernel
//
// Set in sysctl.conf
// fs.file-max=100000
// vm.overcommit_memory = 1
// vm.swappiness=0
// net.ipv4.tcp_sack=1                   # enable selective acknowledgements
// net.ipv4.tcp_timestamps=1             # needed for selective acknowledgements
// net.ipv4.tcp_window_scaling=1         # scale the network window
// net.ipv4.tcp_congestion_control=cubic # better congestion algorithm
// net.ipv4.tcp_syncookies=1             # enable syn cookies
// net.ipv4.tcp_tw_recycle=1             # recycle sockets quickly
// net.ipv4.tcp_max_syn_backlog=NUMBER   # backlog setting
// net.core.somaxconn=10000              # up the number of connections per port
// #net.core.rmem_max=NUMBER              # up the receive buffer size
// #net.core.wmem_max=NUMBER              # up the buffer size for all connections
// echo 1 > /proc/sys/net/ipv4/tcp_tw_reuse
// echo 1 > /proc/sys/net/ipv4/tcp_tw_recycle
// echo 10000 > /proc/sys/net/core/somaxconn
// echo 'never' > /sys/kernel/mm/transparent_hugepage/enabled
// redis.conf
// maxmemory-policy noeviction
// notify-keyspace-events "KEx"
// tcp-backlog 65536

// Package mika is the executable of mika
package main

import (
	"flag"
	"fmt"
	"github.com/Goon3r/iceMika"
	"github.com/Goon3r/iceMika/conf"
	"github.com/Goon3r/iceMika/db"
	"github.com/Goon3r/iceMika/geo"
	"github.com/Goon3r/iceMika/tracker"
	"github.com/Goon3r/iceMika/util"
	log "github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/sentry"
	"github.com/gin-gonic/gin"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
)

var (
	profile     = flag.String("profile", "", "write cpu profile to file")
	config_file = flag.String("config", "./config.json", "Config file path")
	num_procs   = flag.Int("procs", runtime.NumCPU()-1, "Number of CPU cores to use (default: ($num_cores-1))")
)

// sigHandler watches for external signals and will act on them
// Currently only the following signals are implemented:
//
// - SIGINT cleanly exists the running process
// - SIGUSR2 reload the configuration file
func sigHandler(s chan os.Signal) {
	for received_signal := range s {
		switch received_signal {
		case syscall.SIGINT:
			log.Warn("CAUGHT SIGINT: Shutting down!")
			if *profile != "" {
				log.Println("> Writing out profile info")
				pprof.StopCPUProfile()
			}
			util.CaptureMessage("Stopped tracker")
			os.Exit(0)
		case syscall.SIGUSR2:
			log.Warn("CAUGHT SIGUSR2: Reloading config")
			<-s
			conf.LoadConfig(*config_file, false)
			log.Info("> Reloaded config")
			util.CaptureMessage("Reloaded configuration")
		}
	}
}

// main initialized all models and starts the tracker service
func main() {
	log.Info(fmt.Sprintf("Mika (%s) Started Successfully", mika.Version))
	log.Info("Process ID:", os.Getpid())
	log.Info("Num procs(s):", *num_procs)

	// Set max number of CPU cores to use
	runtime.GOMAXPROCS(*num_procs)

	if *profile != "" {
		f, err := os.Create(*profile)
		if err != nil {
			log.Fatal("Could not create profile:", err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	conf.LoadConfig(*config_file, true)

	if conf.Config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	logger := log.New()

	mika.SetupSentry()

	hook, err := logrus_sentry.NewSentryHook(conf.Config.SentryDSN, []log.Level{
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
	})

	if err == nil {
		logger.Hooks.Add(hook)
	}

	mika.SetupLogger(conf.Config.LogLevel, conf.Config.ColourLogs)

	// Start stat counter
	//stats.Setup(conf.Config.MetricsDSN)

	db.Setup(conf.Config.RedisHost, conf.Config.RedisPass)

	geo.Setup(conf.Config.GeoDBPath)

	tracker.Mika = tracker.NewTracker()
	tracker.Mika.Initialize()
	tracker.Mika.Run()
}

func init() {
	// Parse CLI args
	flag.Parse()

	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGUSR2, syscall.SIGINT)
	go sigHandler(s)
}
