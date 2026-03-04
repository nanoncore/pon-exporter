package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kingpin/v2"
	"github.com/nanoncore/pon-exporter/internal/collector"
	"github.com/nanoncore/pon-exporter/internal/config"
	"github.com/nanoncore/pon-exporter/internal/poller"
	"github.com/nanoncore/pon-exporter/internal/target"
	"github.com/nanoncore/pon-exporter/internal/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	toolkit "github.com/prometheus/exporter-toolkit/web"
)

func main() {
	app := kingpin.New("pon-exporter", "Prometheus exporter for GPON optical signal monitoring.")
	configFile := app.Flag("config.file", "Path to configuration file.").Default("pon-exporter.yml").String()
	listenAddress := app.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9876").String()
	webConfigFile := app.Flag("web.config.file", "Path to TLS/basic-auth configuration (exporter-toolkit).").Default("").String()
	logLevel := app.Flag("log.level", "Log level (debug, info, warn, error).").Default("info").Enum("debug", "info", "warn", "error")
	logFormat := app.Flag("log.format", "Log format (logfmt, json).").Default("logfmt").Enum("logfmt", "json")
	app.Version(fmt.Sprintf("%s (revision=%s, branch=%s, built=%s)", version.Version, version.Revision, version.Branch, version.BuildDate))
	app.HelpFlag.Short('h')

	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Setup logger
	logger := setupLogger(*logLevel, *logFormat)

	logger.Info("starting pon-exporter",
		"version", version.Version,
		"revision", version.Revision,
	)

	// Load config
	cfg, err := config.Load(*configFile)
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}
	logger.Info("config loaded", "targets", len(cfg.Targets))

	// Setup snapshot store, collector, registry
	store := poller.NewSnapshotStore()
	gponcol := collector.New(store)
	reg := prometheus.NewRegistry()
	reg.MustRegister(gponcol)
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// Start poller
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	mgr := poller.NewManager(store, target.Poll, logger)
	mgr.Start(ctx, cfg)

	// Setup HTTP
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	mux.HandleFunc("/-/healthy", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})
	mux.HandleFunc("/-/ready", func(w http.ResponseWriter, _ *http.Request) {
		if store.HasData() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "OK")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "Not ready — waiting for first poll")
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>PON Exporter</title></head><body>
<h1>PON Exporter</h1>
<p><a href="/metrics">Metrics</a></p>
<p><a href="/-/healthy">Health</a></p>
<p><a href="/-/ready">Ready</a></p>
</body></html>`)
	})

	server := &http.Server{
		Addr:    *listenAddress,
		Handler: mux,
	}

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Start server
	go func() {
		logger.Info("listening", "address", *listenAddress)
		var srvErr error
		if *webConfigFile != "" {
			flagsMap := &toolkit.FlagConfig{
				WebListenAddresses: &[]string{*listenAddress},
				WebConfigFile:      webConfigFile,
			}
			srvErr = toolkit.ListenAndServe(server, flagsMap, logger)
		} else {
			srvErr = server.ListenAndServe()
		}
		if srvErr != nil && srvErr != http.ErrServerClosed {
			logger.Error("server error", "err", srvErr)
			os.Exit(1)
		}
	}()

	// Wait for signals
	for {
		sig := <-sigCh
		switch sig {
		case syscall.SIGHUP:
			logger.Info("received SIGHUP, reloading config")
			newCfg, err := config.Load(*configFile)
			if err != nil {
				logger.Error("failed to reload config", "err", err)
				continue
			}
			mgr.Reload(ctx, newCfg)
			logger.Info("config reloaded", "targets", len(newCfg.Targets))
		case syscall.SIGINT, syscall.SIGTERM:
			logger.Info("received shutdown signal", "signal", sig)
			mgr.Stop()
			if err := server.Shutdown(context.Background()); err != nil {
				logger.Error("server shutdown error", "err", err)
			}
			return
		}
	}
}

func setupLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(handler)
}
