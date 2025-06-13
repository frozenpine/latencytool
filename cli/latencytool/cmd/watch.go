/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"time"

	"github.com/frozenpine/latency4go"
	"github.com/spf13/cobra"
)

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Monitoring exchange's fronts latency",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		interval, _ := cmd.Flags().GetDuration("interval")
		once, _ := cmd.Flags().GetBool("once")
		if once {
			interval = 0
		}

		if ins := client.Load(); ins != nil {
			if err := ins.Start(interval); err != nil {
				return err
			}

			return ins.Join()
		}

		return errInvalidInstance
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)

	watchCmd.Flags().Duration(
		"interval", time.Minute, "Override default interval arg",
	)
	watchCmd.Flags().IntVar(
		&config.DataSize, "data", 0, "Specify return data size",
	)
	watchCmd.Flags().Bool(
		"once", false, "Run watcher once, conflict & override interval",
	)
	watchCmd.Flags().Var(
		&config.TimeRange, "range",
		"Time range kwargs[key=value] seperated by "+
			latency4go.TIMERANGE_KW_SPLIT,
	)
}
