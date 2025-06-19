package latency4go

type State struct {
	AddrList    []string
	LatencyList []*ExFrontLatency
	Config      QueryConfig
}

func NewState(cfg *QueryConfig, latency []*ExFrontLatency) *State {
	state := State{
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
