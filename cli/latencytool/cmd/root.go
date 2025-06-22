/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/frozenpine/latency4go"
	"github.com/frozenpine/latency4go/cli/latencytool/tui"
	"github.com/frozenpine/latency4go/ctl"
)

var (
	version, goVersion, gitVersion, buildTime string
	verbose                                   int
	logFile                                   string
	logSize                                   int
	logKeep                                   int

	cmdCtx    context.Context
	cmdCancel context.CancelFunc

	client     atomic.Pointer[latency4go.LatencyClient]
	controller atomic.Pointer[ctl.CtlServer]
	config     latency4go.QueryConfig = latency4go.DefaultQueryConfig
)

var (
	errInvalidInstance = errors.New("invalid instance")
	errInvalidArgs     = errors.New("invalid args")
)

func consoleExecute(
	ctx context.Context, client ctl.CtlClient,
	cmdFlags *pflag.FlagSet, cancel func(),
) error {
	defer cancel()

	command, _ := cmdFlags.GetString("cmd")
	if command == "" {
		return errors.New("no command specified")
	}

	client.Init(ctx, "ctl client", client.Start)

	execute := ctl.Command{
		Name:   command,
		KwArgs: map[string]string{},
	}

	switch command {
	case "suspend":
	case "resume":
	case "period":
		if cmdFlags.Changed("interval") {
			interv, _ := cmdFlags.GetDuration("interval")
			execute.KwArgs["interval"] = interv.String()
		} else {
			return errors.Join(
				errInvalidArgs,
				errors.New("invalid interval"),
			)
		}
	case "state":
	case "query":
		data, err := json.Marshal(config)
		if err != nil {
			return err
		}
		execute.KwArgs["config"] = string(data)
	case "config":
		if cmdFlags.Changed("before") {
			execute.KwArgs["before"] = config.TimeRange.String()
		}

		if cmdFlags.Changed("range") {
			execute.KwArgs["range"] = config.TimeRange.String()
		}

		if cmdFlags.Changed("from") {
			execute.KwArgs["from"] = strconv.Itoa(config.Tick2Order.From)
		}

		if cmdFlags.Changed("to") {
			execute.KwArgs["to"] = strconv.Itoa(config.Tick2Order.To)
		}

		if cmdFlags.Changed("percents") {
			execute.KwArgs["percents"] = config.Quantile.String()
		}

		if cmdFlags.Changed("agg") {
			execute.KwArgs["agg"] = strconv.Itoa(config.AggSize)
		}

		if cmdFlags.Changed("least") {
			execute.KwArgs["least"] = strconv.Itoa(config.AggCount)
		}

		if cmdFlags.Changed("user") {
			execute.KwArgs["user"] = config.Users.String()
		}

		if cmdFlags.Changed("sort") {
			execute.KwArgs["sort"] = config.SortBy
		}
	case "plugin":
	case "unplugin":
	default:
		return errors.New("unsupported command")
	}

	wait := make(chan struct{})

	client.MessageLoop(
		"console loop",
		nil, nil,
		func(r *ctl.Result) error {
			defer func() {
				if r.CmdName == command {
					close(wait)
				}
			}()

			return ctl.LogResult(r)
		},
		nil,
	)

	if err := client.Command(&execute); err != nil {
		return err
	}

	<-wait

	return nil
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "latencytool",
	Short: "Trading latency monitoring & reporting",
	Long: `Check & sort exchange's fronts by latency(onetime | periodically), 
and report to trading systems`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	SilenceUsage:  true,
	SilenceErrors: true,
	Version: fmt.Sprintf(
		"%s, Commit: %s, Build: %s@%s",
		version, gitVersion, buildTime, goVersion,
	),
	RunE: func(cmd *cobra.Command, args []string) error {
		clientConn, _ := cmd.Flags().GetString("conn")
		useTui, _ := cmd.Flags().GetBool("tui")

		var (
			client ctl.CtlClient
			err    error
		)

		if clientConn != "" {
			switch {
			case strings.HasPrefix(clientConn, "ipc://"):
				client, err = ctl.NewCtlIpcClient(
					strings.TrimPrefix(clientConn, "ipc://"),
				)
			case strings.HasPrefix(clientConn, "tcp://"):
				client, err = ctl.NewCtlTcpClient(
					strings.TrimPrefix(clientConn, "tcp://"),
				)
			default:
				client, err = ctl.NewCtlIpcClient(clientConn)
			}

			if err != nil {
				return err
			}

			if useTui {
				err = tui.StartTui(
					cmdCtx, client, cmd.Flags(), client.Release,
				)
			} else {
				err = consoleExecute(
					cmdCtx, client, cmd.Flags(), client.Release,
				)
			}

			client.Join()
			return err
		} else {
			return cmd.Help()
		}
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == cmd.Root().Name() {
			return nil
		}

		slog.Info("pre run initiating latency client")

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

		ctlConns, _ := cmd.Flags().GetStringSlice("ctl")
		if len(ctlConns) > 0 {
			cfg := &ctl.CtlSvrHdlConfig{}
			for _, conn := range ctlConns {
				switch {
				case strings.HasPrefix(conn, "ipc://"):
					cfg = cfg.Ipc(conn)
				case strings.HasPrefix(conn, "tcp://"):
					cfg = cfg.Tcp(conn)
				default:
					cfg = cfg.Ipc(conn)
				}

				if cfg == nil {
					return errors.New("create ctl handler failed")
				}
			}

			if svr, err := ctl.NewCtlServer(cmdCtx, cfg); err != nil {
				return err
			} else if err = svr.Start(&client); err != nil {
				return err
			} else {
				controller.Store(svr)
			}
		} else {
			slog.Warn("no ctl handler specified, run w/o ctl server")
		}

		slog.Info("pre run latency client initiated")
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == cmd.Root().Name() {
			return nil
		}

		slog.Info("post run cleanning resources")

		if svr := controller.Load(); svr != nil {
			svr.Stop()

			if err := svr.Join(); err != nil {
				slog.Error(
					"stop controller server failed",
					slog.Any("error", err),
				)
			} else {
				slog.Info("ctl server stopped")
			}
		}

		if ins := client.Load(); ins != nil {
			ins.Stop()

			if err := ins.Join(); err != nil {
				return err
			}

			slog.Info("latency client stopped")
			return nil
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

func initLog() {
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

		logWr = io.MultiWriter(logWr, tui.LogWriter())
	} else {
		logWr = tui.LogWriter()
	}

	if verbose > 0 {
		level = slog.LevelDebug - slog.Level(verbose-1)
		addSource = true
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(
		logWr, &slog.HandlerOptions{
			AddSource: addSource,
			Level:     level,
		},
	)))

	slog.Debug("logger initiated")
}

func init() {
	cobra.OnInitialize(initLog)

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

	rootCmd.PersistentFlags().String(
		"sink", "", "Sink latency data for next cold start",
	)
	rootCmd.PersistentFlags().Var(
		&config.TimeRange, "before", "Lantency doc time range before now",
	)
	rootCmd.PersistentFlags().Var(
		&config.TimeRange, "range",
		"Time range kwargs[key=value] seperated by "+
			latency4go.TIMERANGE_KW_SPLIT,
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

	rootCmd.PersistentFlags().StringSlice(
		"ctl", nil, "Control service listen string",
	)
	rootCmd.Flags().String(
		"conn", "", "Control service connect string",
	)
	rootCmd.Flags().Bool(
		"tui", false, "Use TUI for ctl client console",
	)
	rootCmd.Flags().String(
		"cmd", "", "Command for ctl server handle",
	)

	for _, cmd := range rootCmd.Commands() {
		cmd.Version = rootCmd.Version
	}
}
