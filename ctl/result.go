package ctl

import (
	"encoding/json"
	"log/slog"
	"slices"

	"github.com/frozenpine/latency4go"
)

type resultValueKey string

const (
	VKeyInterval       resultValueKey = "Interval"
	VKeyIntervalOrigin resultValueKey = "Origin"
	VKeyState          resultValueKey = "State"
	VKeyConfig         resultValueKey = "Config"
	VKeyPlugin         resultValueKey = "Plugins"
	VKeyHandler        resultValueKey = "Handlers"
)

type values map[resultValueKey]any

type Result struct {
	Rtn     int
	Message string
	CmdName string
	Values  values
}

func (r *Result) UnmarshalJSON(v []byte) error {
	results := make(map[resultValueKey]json.RawMessage)

	if err := json.Unmarshal(v, &results); err != nil {
		return err
	}

	if err := json.Unmarshal(results["Rtn"], &r.Rtn); err != nil {
		return err
	}

	if err := json.Unmarshal(results["Message"], &r.Message); err != nil {
		return err
	}

	if err := json.Unmarshal(results["CmdName"], &r.CmdName); err != nil {
		return err
	}

	if len(results["Values"]) > 0 {
		data := make(map[resultValueKey]json.RawMessage)
		r.Values = values{}
		if err := json.Unmarshal(results["Values"], &data); err != nil {
			return nil
		}

		for k, v := range data {
			r.Values[k] = v
		}
	}

	return nil
}

var LogResult = func(result *Result) error {
	values := values{}
	keys := make([]resultValueKey, 0, len(result.Values))

	for k, v := range result.Values {
		var value any

		if err := json.Unmarshal(
			v.(json.RawMessage), &value,
		); err != nil {
			slog.Error(
				"unmarshal result values failed",
				slog.Any("error", err),
				slog.String("key", string(k)),
			)
		} else {
			values[k] = value
		}

		keys = append(keys, k)
	}

	slices.Sort(keys)
	attrs := append(
		[]any{
			slog.String("cmd", result.CmdName),
			slog.String("cmd_msg", result.Message),
		},
		latency4go.ConvertSlice(keys, func(k resultValueKey) any {
			return slog.Any(string(k), values[k])
		})...,
	)

	var logger func(string, ...any)
	if result.Rtn == 0 {
		logger = slog.Info
	} else {
		logger = slog.Error
	}

	logger(
		"command result received",
		attrs...,
	)

	return nil
}

var LogState = func(state *latency4go.State) error {
	slog.Info(
		"latency state notified",
		slog.Time("update_ts", state.Timestamp),
		slog.String("config", state.Config.String()),
		slog.Any("latency", state.LatencyList),
		slog.Any("priority", state.AddrList),
	)

	return nil
}
