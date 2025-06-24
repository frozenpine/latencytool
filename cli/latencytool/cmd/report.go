/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/frozenpine/latency4go"
	"github.com/frozenpine/latency4go/libs"
	"github.com/spf13/cobra"
	"github.com/valyala/bytebufferpool"
)

const DEFAULT_PLUGIN_LIB_DIR = "libs"

type pluginConfigs map[string]string

func (c *pluginConfigs) Set(v string) error {
	if c == nil || *c == nil {
		*c = make(pluginConfigs)
	}

	values := latency4go.ConvertSlice(
		strings.SplitN(v, "=", 2),
		func(v string) string { return strings.TrimSpace(v) },
	)

	if len(values) != 2 {
		return errInvalidArgs
	}

	(*c)[values[0]] = values[1]

	return nil
}

func (c pluginConfigs) Type() string {
	return "PluginConfig"
}

func (c pluginConfigs) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	return buff.String()
}

var (
	configs = make(pluginConfigs)

	libDir string
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Watching latency & report to trading system",
	Long: `Watching exchange's fronts latency, reporting result to 
trading systems specified by args.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		plugins, _ := cmd.Flags().GetStringSlice("plugin")
		if len(plugins) < 1 {
			return errors.New("no plugin specified")
		}

		for idx, name := range plugins {
			cfg, exists := configs[name]

			if !exists {
				return fmt.Errorf(
					"%w: no config specified for plugin %s",
					errInvalidArgs, name,
				)
			}

			container, err := libs.NewPlugin(libDir, name)
			if err != nil {
				return err
			}

			slog.Info(
				"initializing plugin",
				slog.String("name", name),
			)

			if err := container.Init(cmdCtx, cfg); err != nil {
				return err
			}

			// 修正plugin实际名称
			plugins[idx] = container.Name()
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		once, _ := cmd.Flags().GetBool("once")

		if once || config.TimeRange[latency4go.TimeFrom] != "" {
			slog.Info(
				"args confilicted with --interval, set to onetime running",
				slog.Bool("once", once),
				slog.String("range", config.TimeRange.String()),
			)
			interval = 0
		}

		if ins := client.Load(); ins != nil {
			if err := libs.RangePlugins(func(
				name string, container *libs.PluginContainer,
			) error {
				if err := ins.AddReporter(
					name, func(s *latency4go.State) error {
						return container.ReportFronts(s.AddrList...)
					},
				); err != nil {
					return err
				}

				slog.Info(
					"plugin reporter registered",
					slog.String("plugin", container.String()),
				)
				return nil
			}); err != nil {
				return err
			}

			if err := ins.Start(interval); err != nil {
				return err
			}

			return ins.Join()
		}

		return errInvalidInstance
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		libs.RangePlugins(func(
			name string, container *libs.PluginContainer,
		) error {
			slog.Info(
				"trying to stop plugin",
				slog.String("plugin", container.String()),
			)

			container.Stop()

			if err := container.Join(); err != nil {
				slog.Error(
					"plugin stop failed",
					slog.Any("error", err),
					slog.String("plugin", container.String()),
				)
			} else {
				slog.Info(
					"plugin stopped",
					slog.String("plugin", container.String()),
				)
			}

			return nil
		})
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().StringVar(
		&libDir, "lib", DEFAULT_PLUGIN_LIB_DIR, "Reporter plugin lib dir",
	)
	reportCmd.Flags().StringSlice(
		"plugin", nil,
		"Reporter plugin's name, loaded from ${lib}/${plugin}/${plugin}.{ext}",
	)
	reportCmd.Flags().Var(
		&configs, "config",
		"Reporter plugin's config file path, ${plugin}=PATH",
	)

	reportCmd.Flags().Duration(
		"interval", time.Minute, "Override global interval arg",
	)
	reportCmd.Flags().Bool(
		"once", false, "Run watcher once, conflict & override interval",
	)
}
