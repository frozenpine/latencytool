/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/frozenpine/latency4go"
)

var (
	version, goVersion, gitVersion, buildTime string
	verbose                                   int
	logFile                                   string
	logSize                                   int
	logKeep                                   int

	cmdCtx    context.Context
	cmdCancel context.CancelFunc

	client atomic.Pointer[latency4go.LatencyClient]
	config latency4go.QueryConfig = latency4go.QueryConfig{
		TimeRange: latency4go.TimeRange{From: "1m", To: "now"},
	}
)

var (
	errInvalidInstance = errors.New("invalid instance")
	errInvalidArgs     = errors.New("invalid args")
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "latencytool",
	Short: "Trading latency monitoring & reporting",
	Long: `Check & sort exchange's fronts by latency(onetime | periodically), 
and report to trading systems`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
	Version: fmt.Sprintf(
		"%s, Commit: %s, Build: %s@%s",
		version, gitVersion, buildTime, goVersion,
	),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == cmd.Root().Name() {
			return nil
		}

		schema, _ := cmd.Flags().GetString("schema")
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		sink, _ := cmd.Flags().GetString("sink")

		ins := latency4go.LatencyClient{}

		if err := ins.Init(
			cmdCtx, schema, host, port, sink, &config,
		); err != nil {
			return errors.Join(err, errInvalidArgs, errInvalidInstance)
		}

		client.Store(&ins)

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == cmd.Root().Name() {
			return nil
		}

		if ins := client.Load(); ins != nil {
			ins.Stop()

			return ins.Join()
		} else {
			return errInvalidInstance
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	defer cmdCancel()

	if err := rootCmd.Execute(); err != nil {
		slog.Error(
			"latency tool run failed",
			slog.Any("error", err),
		)

		os.Exit(1)
	}
}

func initConfig() {
	var (
		level     = slog.LevelInfo
		addSource bool
		logWr     io.Writer
		err       error
	)

	if logFile != "" {
		if logSize > 0 {
			logWr = &lumberjack.Logger{
				Filename: logFile,
				MaxSize:  logSize,
				MaxAge:   logKeep,
				Compress: true,
			}
		} else if logWr, err = os.OpenFile(
			logFile,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			os.ModePerm,
		); err != nil {
			panic(err)
		}

		logWr = io.MultiWriter(logWr, os.Stderr)
	} else {
		logWr = os.Stderr
	}

	if verbose > 0 {
		level = slog.LevelDebug
		addSource = true
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(
		logWr, &slog.HandlerOptions{
			AddSource: addSource,
			Level:     level,
		},
	)))
}

func init() {
	cobra.OnInitialize(initConfig)

	// version string for all sub commands
	for _, cmd := range rootCmd.Commands() {
		cmd.Version = rootCmd.Version
	}

	// signal handler for global context
	cmdCtx, cmdCancel = signal.NotifyContext(
		context.Background(),
		syscall.SIGKILL, syscall.SIGABRT, syscall.SIGINT,
		syscall.SIGQUIT, syscall.SIGTERM, os.Interrupt,
	)

	rootCmd.PersistentFlags().CountVarP(
		&verbose, "verbose", "v", "Running in debug mode")
	rootCmd.PersistentFlags().StringVar(
		&logFile, "log", "", "Log file path")
	rootCmd.PersistentFlags().IntVar(
		&logSize, "size", 300, "Log rotate size in MB")
	rootCmd.PersistentFlags().IntVar(
		&logKeep, "keep", 5, "Log archive keep count")

	rootCmd.PersistentFlags().String(
		"schema", "http", "Latency system's request schema",
	)
	rootCmd.PersistentFlags().String(
		"host", "10.36.51.124", "Latency system's request host",
	)
	rootCmd.PersistentFlags().Int(
		"port", 9200, "Latency system's request port",
	)
	rootCmd.PersistentFlags().Duration(
		"interval", 0, "Run periodically interver, 0 for onetime running",
	)

	rootCmd.PersistentFlags().Var(
		&config.TimeRange, "before", "Lantency",
	)
	rootCmd.PersistentFlags().IntVar(
		&config.Tick2Order.From, "from", 0,
		"Tick2Order range left bound in PicoSec",
	)
	rootCmd.PersistentFlags().IntVar(
		&config.Tick2Order.To, "to", 100000000,
		"Tick2Order range right bound in PicoSec",
	)
	rootCmd.PersistentFlags().Float64SliceVar(
		(*[]float64)(&config.Quantile), "percents",
		[]float64{10, 25, 50, 75, 90},
		"Latency aggregation quantile percents",
	)
	rootCmd.PersistentFlags().IntVar(
		&config.AggSize, "agg", 15, "Aggregation results size",
	)
	rootCmd.PersistentFlags().IntVar(
		&config.AggCount, "least", 5, "At least doc count for aggregation",
	)
	rootCmd.PersistentFlags().StringSliceVar(
		(*[]string)(&config.Users), "user", nil,
		"ClientID filter for aggregation",
	)
	rootCmd.PersistentFlags().StringVar(
		&config.SortBy, "sort", "",
		"Sort exchange's fronts by elastic painless",
	)

	for _, cmd := range rootCmd.Commands() {
		cmd.Version = rootCmd.Version
	}
}
