package ctl

import (
	"encoding/json"
	"log/slog"
	"slices"

	"github.com/frozenpine/latency4go"
)

type Result struct {
	Rtn     int
	Message string
	CmdName string
	Values  map[string]any
}

func (r *Result) UnmarshalJSON(v []byte) error {
	data := make(map[string]json.RawMessage)

	if err := json.Unmarshal(v, &data); err != nil {
		return err
	}

	if err := json.Unmarshal(data["Rtn"], &r.Rtn); err != nil {
		return err
	}

	if err := json.Unmarshal(data["Message"], &r.Message); err != nil {
		return err
	}

	if err := json.Unmarshal(data["CmdName"], &r.CmdName); err != nil {
		return err
	}

	values := make(map[string]json.RawMessage)
	r.Values = make(map[string]any)
	if err := json.Unmarshal(data["Values"], &values); err != nil {
		return nil
	}

	for k, v := range values {
		r.Values[k] = v
	}

	return nil
}

var LogResult = func(result *Result) error {
	values := map[string]any{}
	keys := make([]string, 0, len(result.Values))

	for k, v := range result.Values {
		var value any

		if err := json.Unmarshal(
			v.(json.RawMessage), &value,
		); err != nil {
			slog.Error(
				"unmarshal result values failed",
				slog.Any("error", err),
				slog.String("key", k),
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
		latency4go.ConvertSlice(keys, func(k string) any {
			return slog.Any(k, values[k])
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
