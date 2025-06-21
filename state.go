package latency4go

import "time"

type State struct {
	Timestamp   time.Time
	AddrList    []string
	LatencyList []*ExFrontLatency
	Config      QueryConfig
}

func NewState(ts time.Time, cfg *QueryConfig, latency []*ExFrontLatency) *State {
	state := State{
		Timestamp:   ts,
		LatencyList: make([]*ExFrontLatency, len(latency)),
		Config:      *cfg,
	}
	copy(state.LatencyList, latency)
	state.AddrList = ConvertSlice(
		state.LatencyList,
		func(v *ExFrontLatency) string {
			return v.FrontAddr
		},
	)

	return &state
}
