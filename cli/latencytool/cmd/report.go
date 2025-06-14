/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"

	"github.com/frozenpine/latency4go/cli/latencytool/libs"
	"github.com/spf13/cobra"
)

const PLUGIN_LIB_DIR = "libs"

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Watching latency & report to trading system",
	Long: `Watching exchange's fronts latency, reporting result to 
trading systems specified by args.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		libNames, _ := cmd.Flags().GetStringSlice("lib")

		rptCtx, rptCancel := context.WithCancel(cmdCtx)
		defer rptCancel()

		plugins := make([]*libs.PluginLib, 0, len(libNames))

		for _, name := range libNames {
			libPath := fmt.Sprintf(
				"%s/%s/%s.plugin", PLUGIN_LIB_DIR,
				name, name,
			)

			plugin, err := libs.NewPlugin(rptCtx, libPath)

			if err != nil {
				return err
			}

			plugins = append(plugins, plugin)
		}

		for _, plugin := range plugins {
			plugin.Join()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().StringSlice(
		"lib", nil, "Reporter plugin lib",
	)
}
