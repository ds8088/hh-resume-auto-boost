package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func setupLogger(ctx *AppContext) {
	level := slog.LevelInfo
	if ctx.Cfg.Debug {
		level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	slog.SetDefault(logger)
	slog.Debug("initialized logger")
}

func runApp(ctx *AppContext, configPath string) error {
	var cancel context.CancelFunc
	ctx.Context, cancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err := ctx.Cfg.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	err = ctx.Cfg.Validate()
	if err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	setupLogger(ctx)

	cl := createHTTPClient(ctx)

	sched := newResumeScheduler()
	defer sched.teardown()

	// Main logic loop: repeatedly discover resumes and schedule them
	for resume := range discoverResumes(ctx, cl) {
		sched.schedule(ctx, cl, resume)
	}

	// When there's nothing to discover anymore,
	// wait until we're cancelled,
	// or when the scheduler says that it also has nothing to schedule
	select {
	case <-sched.done():
	case <-ctx.Done():
		slog.Debug("shutting down due to context cancellation")
	}

	return nil
}

func main() {
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()

	ctx := &AppContext{}
	ctx.Cfg.Instantiate()

	version := "1.0.0"

	var configPath string
	flag.StringVar(&configPath, "c", "config.json", "")
	flag.StringVar(&configPath, "config", "config.json", "")

	flag.BoolVar(&ctx.Cfg.Debug, "d", false, "")
	flag.BoolVar(&ctx.Cfg.Debug, "debug", false, "")
	flag.StringVar(&ctx.Cfg.Login, "l", "", "")
	flag.StringVar(&ctx.Cfg.Login, "login", "", "")
	flag.StringVar(&ctx.Cfg.Password, "p", "", "")
	flag.StringVar(&ctx.Cfg.Password, "password", "", "")

	flag.Usage = func() {
		fmt.Println("hh-resume-auto-boost v" + version + ": automatically boosts HeadHunter resumes\n" +
			"Copyright (C) 2025 Dave S.\n\n" +
			"Usage:\n" +
			"\t-d, --debug: enable debug output\n" +
			"\t-c, --config: path to JSON-formatted config file (default: \"config.json\")\n" +
			"\t-l, --login: HeadHunter username (email, phone or login)\n" +
			"\t-p, --password: HeadHunter password. Insecure; use config instead")
	}

	flag.Parse()

	defer func() {
		if r := recover(); r != nil {
			var errorText string

			switch rr := r.(type) {
			case error:
				errorText = rr.Error()
			case string:
				errorText = rr
			}

			slog.Error("terminating due to an uncaught error", "error", errorText)
			exitCode = 1
		}
	}()

	err := runApp(ctx, configPath)
	if err != nil {
		slog.Error(err.Error())
		exitCode = 1
	}

	slog.Info("shutting down")
}
