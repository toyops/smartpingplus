package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/cihub/seelog"
	"github.com/jakecoffman/cron"
	"github.com/smartping/smartping/src/funcs"
	"github.com/smartping/smartping/src/g"
	"github.com/smartping/smartping/src/http"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Init config
var Version = "0.9.1"

func main() {
	version := flag.Bool("v", false, "show version")
	flag.Parse()
	if *version {
		fmt.Println(Version)
		os.Exit(0)
	}
	g.ParseConfig(Version)
	go funcs.ClearArchive()
	c := cron.New()
	c.AddFunc("*/60 * * * * *", func() {
		go funcs.Ping()
		go funcs.TcpPing()
		go funcs.Mapping()
		if g.Cfg.Mode["Type"] == "cloud" {
			go funcs.StartCloudMonitor()
		}
	}, "ping")
	c.AddFunc("0 0 * * * *", func() {
		go funcs.ClearArchive()
	}, "mtc")
	c.Start()

	server := http.InitHttpServer()
	go func() {
		seelog.Info("[StartHttp] starting to listen on ", g.Cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != nethttp.ErrServerClosed {
			seelog.Critical("[StartHttp] server error: ", err)
		}
	}()

	// Graceful shutdown on SIGINT or SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	seelog.Info("[Shutdown] Received signal: ", sig, ", initiating graceful shutdown...")

	// 1. Stop cron scheduler
	c.Stop()
	seelog.Info("[Shutdown] Cron scheduler stopped")

	// 2. Shutdown HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		seelog.Error("[Shutdown] HTTP server shutdown error: ", err)
	} else {
		seelog.Info("[Shutdown] HTTP server stopped")
	}

	// 3. Close database connection cleanly
	if g.Db != nil {
		if err := g.Db.Close(); err != nil {
			seelog.Error("[Shutdown] Database close error: ", err)
		} else {
			seelog.Info("[Shutdown] Database connection closed")
		}
	}

	seelog.Flush()
	seelog.Info("[Shutdown] SmartPing shutdown cleanly")
}
