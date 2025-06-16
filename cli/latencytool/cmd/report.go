/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"github.com/frozenpine/latency4go"
	"github.com/frozenpine/latency4go/cli/latencytool/libs"
	"github.com/spf13/cobra"
	"github.com/valyala/bytebufferpool"
)

const PLUGIN_LIB_DIR = "libs"

var (
	libDir string
)

type pluginCache map[string]libs.Plugin

func (p *pluginCache) Set(name string) error {
	if p == nil || *p == nil {
		*p = make(pluginCache)
	}

	var (
		suffix = "plugin"
		plugin libs.Plugin
		err    error
	)

	if strings.HasPrefix(name, "C.") {
		suffix = "so"
		name = strings.TrimPrefix(name, "C.")
	} else if strings.ToLower(runtime.GOOS) != "linux" {
		// go plugin only supported in linux
		// double check environment for correct plugin type
		suffix = "so"
	}

	switch suffix {
	case "plugin":
		plugin, err = libs.NewPlugin(libDir, name)
	case "so":
		plugin, err = libs.NewCPlugin(libDir, name)
	default:
		return errInvalidArgs
	}

	if err != nil {
		return errors.Join(errInvalidArgs, err)
	}

	(*p)[name] = plugin

	return nil
}

func (p pluginCache) Type() string {
	return "PluginCache"
}

func (p pluginCache) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	return buff.String()
}

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
	plugins = make(pluginCache)
	configs = make(pluginConfigs)
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Watching latency & report to trading system",
	Long: `Watching exchange's fronts latency, reporting result to 
trading systems specified by args.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		rptCtx, rptCancel := context.WithCancel(cmdCtx)
		defer rptCancel()

		for name, plugin := range plugins {
			cfg, exists := configs[name]

			if !exists {
				return fmt.Errorf(
					"%w: no config specified for plugin %s",
					errInvalidArgs, name,
				)
			}

			slog.Info(
				"initializing plugin",
				slog.String("name", name),
			)

			if err := plugin.Init(rptCtx, cfg); err != nil {
				return err
			}
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		once, _ := cmd.Flags().GetBool("once")

		if once || config.TimeRange[latency4go.TimeFrom] != "" {
			slog.Info(
				"args confilicted with --interval, set to onetime running",
			)
			interval = 0
		}

		if ins := client.Load(); ins != nil {
			for name, plugin := range plugins {
				ins.RegReporter(name, plugin.ReportFronts)
			}

			if err := ins.Start(interval); err != nil {
				return err
			}

			return ins.Join()
		}

		return errInvalidInstance
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		for _, plugin := range plugins {
			plugin.Join()
		}
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().StringVar(
		&libDir, "lib", PLUGIN_LIB_DIR, "Reporter plugin lib dir",
	)
	reportCmd.Flags().Var(
		&plugins, "plugin",
		"Reporter plugin's name, loaded from ${lib}/${plugin}/${plugin}.plugin",
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
