package latency4go

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestQuery(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	client := LatencyClient{}

	if err := client.Init(
		context.WithValue(t.Context(), CTX_VERBOSE_KEY, true),
		"http", "10.36.51.124", 9200, "",
		&QueryConfig{
			Tick2Order: Tick2Order{To: 100000000},
			TimeRange: TimeRange{
				TimeFrom: "2025-06-13T09:30:00+08:00",
				TimeTo:   "2025-06-13T09:40:00+08:00",
			},
			AggSize:  15,
			AggCount: 5,
			Quantile: Quantile{10, 25, 50, 75, 90},
			DataSize: 100,
		},
	); err != nil {
		t.Fatal(err)
	}

	if err := client.Start(time.Second * 10); err != nil {
		t.Fatal(err)
	}

	go func() {
		<-time.After(time.Second * 20)
		client.Stop()
	}()

	if err := client.Join(); err != nil {
		t.Fatal(err)
	}
}
