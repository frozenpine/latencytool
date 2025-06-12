/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	version, goVersion, gitVersion, buildTime string
	verbose                                   bool
	logFile                                   string
	logSize                                   int
	logKeep                                   int

	cmdCtx    context.Context
	cmdCancel context.CancelFunc

	reporterDir string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cli",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	Version: fmt.Sprintf(
		"%s, Commit: %s, Build: %s@%s",
		version, gitVersion, buildTime, goVersion,
	),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if reporterDir == "" {
			return fmt.Errorf(
				"%w: data base DIR not specified", errInvalidArgs,
			)
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
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
	} else {
		logWr = os.Stderr
	}

	if verbose {
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

	rootCmd.PersistentFlags().BoolVarP(
		&verbose, "verbose", "v", false, "Running in debug mode")
	rootCmd.PersistentFlags().StringVar(
		&logFile, "log", "", "Log file path")
	rootCmd.PersistentFlags().IntVar(
		&logSize, "size", 500, "Log rotate size in MB")
	rootCmd.PersistentFlags().IntVar(
		&logKeep, "keep", 5, "Log archive keep count")

	for _, cmd := range rootCmd.Commands() {
		cmd.Version = rootCmd.Version
	}

	rootCmd.PersistentFlags().StringVar(
		&reporterDir, "reporter", "", "Reporter lib DIR",
	)
}
