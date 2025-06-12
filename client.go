package latency4go

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/olivere/elastic/v7"
)

var (
	ErrNotInitialized     = errors.New("client not initialized")
	ErrAlreadyInitialized = errors.New("client already initialized")
	ErrAlreadyStarted     = errors.New("client already started")
	ErrParseAggResult     = errors.New("parse aggregation failed")
	ErrInvalidQueryCfg    = errors.New("invalid query config")
	ErrInvalidReporter    = errors.New("invalid reporter")
)

type CTX_KEY string

const CTX_VERBOSE_KEY CTX_KEY = "verbose"

const (
	ELASTIC_DOCUMENTS = "alldelaystatistics202*"
)

type Reporter func(addrList ...string) error

type LatencyClient struct {
	ctx      context.Context
	cancel   context.CancelFunc
	initOnce sync.Once

	addr   string
	client atomic.Pointer[elastic.Client]

	startOnce sync.Once
	runCtx    context.Context
	runCancel context.CancelFunc
	watchRun  chan error
	cfg       atomic.Pointer[QueryConfig]
	notify    chan []*exFrontLatency

	sinkFile     string
	reporterDone sync.WaitGroup
	reporters    sync.Map
}

func (c *LatencyClient) Init(
	ctx context.Context,
	schema, host string, port int,
	sinkFile string, config *QueryConfig,
) (err error) {
	if c.client.Load() != nil {
		return ErrAlreadyInitialized
	}

	if ctx == nil {
		ctx = context.Background()
	}

	c.initOnce.Do(func() {
		c.ctx, c.cancel = context.WithCancel(ctx)

		esLogHandler := slog.Default().Handler().WithGroup("ES")

		c.addr = fmt.Sprintf("%s://%s:%d", schema, host, port)

		options := []elastic.ClientOptionFunc{
			elastic.SetURL(c.addr),
			elastic.SetErrorLog(slog.NewLogLogger(
				esLogHandler, slog.LevelError)),
			elastic.SetInfoLog(slog.NewLogLogger(
				esLogHandler, slog.LevelInfo)),
			elastic.SetSniff(false),
		}

		verbose, ok := ctx.Value(CTX_VERBOSE_KEY).(bool)

		if ok && verbose {
			options = append(options, elastic.SetTraceLog(
				slog.NewLogLogger(
					esLogHandler, slog.LevelDebug)))
		}

		var client *elastic.Client

		if client, err = elastic.NewClient(options...); err != nil {
			c.client.Store(client)
			c.cfg.Store(config)
			c.sinkFile = sinkFile
		}
	})

	return
}

func (c *LatencyClient) GetVersion() (string, error) {
	if ins := c.client.Load(); ins != nil {
		return ins.ElasticsearchVersion(c.addr)
	}

	return "", ErrNotInitialized
}

func (c *LatencyClient) getLatency(cfg *QueryConfig) ([]*exFrontLatency, error) {
	if cfg == nil {
		return nil, ErrInvalidQueryCfg
	}
	qry, agg := cfg.makeQuery()

	qryCtx, qryCancel := context.WithCancel(c.runCtx)
	defer qryCancel()

	rsp, err := c.client.Load().Search(
		ELASTIC_DOCUMENTS,
	).Size(
		cfg.DataSize,
	).Query(
		qry,
	).Aggregation(
		AGGREGATION_RESULTS, agg,
	).Do(qryCtx)

	if err != nil {
		return nil, err
	}

	if rsp.Hits.TotalHits.Value > 0 {
		for _, hit := range rsp.Hits.Hits {
			if v, err := hit.Source.MarshalJSON(); err != nil {
				slog.Error(
					"read hits data failed",
					slog.Any("error", err),
				)
			} else {
				slog.Debug(
					"hits data",
					slog.String("record", string(v)),
				)
			}
		}
	}

	termResults, ok := rsp.Aggregations.Terms(AGGREGATION_RESULTS)
	if !ok {
		slog.Error("parse terms data failed")

		return nil, ErrParseAggResult
	}

	latencyList := []*exFrontLatency{}

	for idx, r := range termResults.Buckets {
		front, ok := r.Key.(string)
		if !ok {
			slog.Error(
				"parse front addr failed",
				slog.Any("key", r.Key),
			)
		}

		max, ok := r.Aggregations.Max(EXCHANGE_LATENCY_MAX)
		if !ok {
			slog.Error(
				"parse latency max failed",
				slog.String("addr", *r.KeyAsString),
				slog.String("key", EXCHANGE_LATENCY_MAX),
			)
			continue
		}
		min, ok := r.Aggregations.Min(EXCHANGE_LATENCY_MIN)
		if !ok {
			slog.Error(
				"parse latency min failed",
				slog.String("addr", *r.KeyAsString),
				slog.String("key", EXCHANGE_LATENCY_MIN),
			)
			continue
		}
		percentiles, ok := r.Aggregations.Percentiles(EXCHANGE_LATENCY_PERCENTS)
		if !ok {
			slog.Error(
				"parse latency percents failed",
				slog.String("addr", *r.KeyAsString),
				slog.String("key", EXCHANGE_LATENCY_PERCENTS),
			)
			continue
		}

		latency := exFrontLatency{
			FrontAddr:  front,
			MaxLatency: *max.Value,
			MinLatency: *min.Value,
			Percents:   make(percentResults),
		}

		for k, v := range percentiles.Values {
			percent, _ := strconv.ParseFloat(k, 64)
			latency.Percents[percent] = v
		}

		slog.Info(
			"latency results",
			slog.Int("rank", idx+1),
			slog.Any("latency", latency),
		)

		latencyList = append(latencyList, &latency)
	}

	return latencyList, nil
}

func (c *LatencyClient) sinkLatency(latency []*exFrontLatency) error {
	if c.sinkFile == "" {
		return nil
	}

	if len(latency) <= 0 {
		slog.Debug("empty latency result, skip sink")

		return nil
	}

	data, err := json.Marshal(latency)
	if err != nil {
		return err
	}

	return os.WriteFile(c.sinkFile, data, os.ModePerm)
}

func (c *LatencyClient) runQuerier(interval time.Duration, timeout time.Duration) error {
	c.runCtx, c.runCancel = context.WithCancel(c.ctx)
	c.watchRun = make(chan error, 1)

	go func() {
		defer func() {
			close(c.notify)
			close(c.watchRun)
			c.runCancel()
		}()

		if interval > 0 {
			slog.Info(
				"latency checker running periodically",
				slog.Duration("interval", interval),
			)
		}

		for {
			select {
			case <-c.runCtx.Done():
				return
			default:
				latency, err := c.getLatency(c.cfg.Load())

				if err != nil {
					slog.Error(
						"query latency failed",
						slog.Any("error", err),
					)
				}

				// 一次性运行，直接退出
				if interval <= 0 {
					return
				}

				select {
				case c.notify <- latency:
					if err := c.sinkLatency(latency); err != nil {
						slog.Error(
							"sink latency list failed",
							slog.Any("error", err),
						)
					}
				case <-time.After(timeout):
					slog.Error(
						"publish latency timedout",
						slog.Duration("timeout", timeout),
					)
				}

				select {
				case <-c.runCtx.Done():
					return
				case <-time.After(interval):
				}
			}
		}
	}()

	return nil
}

func (c *LatencyClient) runReporter() {
	defer func() {
		c.reporters.Range(func(key, value any) bool {
			c.reporterDone.Done()
			return true
		})
	}()

	for latency := range c.notify {
		addrList := ConvertSlice(latency, func(v *exFrontLatency) string {
			return v.FrontAddr
		})

		slog.Info(
			"reporting latency",
			slog.Any("priority", addrList),
		)

		c.reporters.Range(func(key, value any) bool {
			name, ok := key.(string)

			if !ok {
				c.reporters.Delete(key)
				c.reporterDone.Done()
				return true
			}

			reportFn, ok := value.(Reporter)
			if !ok {
				c.reporters.Delete(key)
				c.reporterDone.Done()

				return true
			}

			slog.Info(
				"sending latency results to reporter",
				slog.String("reporter", name),
			)

			if err := reportFn(addrList...); err != nil {
				slog.Error(
					"send latency results to reporter failed",
					slog.String("reporter", name),
				)
			}

			return true
		})
	}

	slog.Info("latency notify channel closed")
}

func (c *LatencyClient) RegReporter(name string, reporter Reporter) error {
	if name == "" || reporter == nil {
		return ErrInvalidReporter
	}

	if !c.reporters.CompareAndSwap(name, nil, reporter) {
		return ErrInvalidReporter
	}

	return nil
}

func (c *LatencyClient) Start(interval time.Duration) (err error) {
	if c.notify != nil {
		return ErrAlreadyStarted
	}

	c.startOnce.Do(func() {
		c.notify = make(chan []*exFrontLatency, 10)

		if err = c.runQuerier(interval, time.Second*5); err != nil {
			go c.runReporter()
		}
	})

	return
}

func (c *LatencyClient) Stop() error {
	c.runCancel()

	c.reporterDone.Wait()

	return <-c.watchRun
}
