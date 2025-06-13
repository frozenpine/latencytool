package latency4go

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
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
	ErrReadResponse       = errors.New("read rsp failed")
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

	startOnce   sync.Once
	stopOnce    sync.Once
	runCtx      context.Context
	runCancel   context.CancelFunc
	watchRun    chan error
	cfg         atomic.Pointer[QueryConfig]
	qryInterval atomic.Pointer[time.Duration]
	notify      chan []*exFrontLatency

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

		options = append(options, elastic.SetTraceLog(
			slog.NewLogLogger(
				esLogHandler, slog.LevelDebug-1)))

		var (
			client    *elastic.Client
			esVersion string
		)

		if client, err = elastic.NewClient(options...); err != nil {
			return
		}

		if esVersion, err = client.ElasticsearchVersion(c.addr); err != nil {
			return
		}

		slog.Info(
			"Latency system's elastic info",
			slog.String("version", esVersion),
		)

		c.client.Store(client)
		c.cfg.Store(config)
		c.sinkFile = sinkFile
	})

	return
}

func (c *LatencyClient) GetVersion() (string, error) {
	if ins := c.client.Load(); ins != nil {
		return ins.ElasticsearchVersion(c.addr)
	}

	return "", ErrNotInitialized
}

func (c *LatencyClient) getLatency() ([]*exFrontLatency, error) {
	cfg := c.cfg.Load()
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
		return nil, errors.Join(ErrReadResponse, err)
	}

	if rsp.Hits.TotalHits.Value > 0 {
		for _, hit := range rsp.Hits.Hits {
			if v, err := hit.Source.MarshalJSON(); err != nil {
				return nil, errors.Join(ErrReadResponse, err)
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
		return nil, fmt.Errorf("%w: get terms failed", ErrParseAggResult)
	}

	latencyList := []*exFrontLatency{}

	for _, r := range termResults.Buckets {
		front, ok := r.Key.(string)
		if !ok {
			return nil, fmt.Errorf(
				"%w: parse front addr failed", ErrParseAggResult,
			)
		}

		percentiles, ok := r.Aggregations.Percentiles(EXCHANGE_LATENCY_PERCENTS)
		if !ok {
			return nil, fmt.Errorf(
				"%w: parse latency percents failed", ErrParseAggResult,
			)
		}

		extra, ok := r.Aggregations.ExtendedStats(EXCHANGE_LATENCY_EXTRA)
		if !ok {
			return nil, fmt.Errorf(
				"%w: parse latency extra failed", ErrParseAggResult,
			)
		}

		pri, ok := r.Aggregations.BucketScript(EXCHANGE_LATENCY_PRIORITY)
		if !ok {
			return nil, fmt.Errorf(
				"%w: parse latency priority failed", ErrParseAggResult,
			)
		}

		latency := exFrontLatency{
			FrontAddr:    front,
			Priority:     *pri.Value,
			MaxLatency:   *extra.Max,
			MinLatency:   *extra.Min,
			AvgLatency:   *extra.Avg,
			VarLatency:   *extra.Variance,
			StdevLatency: *extra.StdDeviation,
			DocCount:     r.DocCount,
			Percents:     make(percentResults),
		}

		if err := json.Unmarshal(
			extra.Aggregations["std_deviation_sampling"],
			&latency.SampleStdevLatency,
		); err != nil {
			return nil, errors.Join(ErrParseAggResult, err)
		}

		for k, v := range percentiles.Values {
			percent, _ := strconv.ParseFloat(k, 64)
			latency.Percents[percent] = v
		}

		latencyList = append(latencyList, &latency)
	}

	slices.SortFunc(latencyList, func(l, r *exFrontLatency) int {
		return cmp.Compare(l.Priority, r.Priority)
	})

	for idx, latency := range latencyList {
		slog.Info(
			"latency results",
			slog.Int("rank", idx+1),
			slog.Any("latency", latency),
		)
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

func (c *LatencyClient) runQuerier(
	timeout time.Duration,
) error {
	c.runCtx, c.runCancel = context.WithCancel(c.ctx)
	c.watchRun = make(chan error, 1)

	// self log reporter
	c.reporterDone.Add(1)

	go func() {
		defer func() {
			close(c.notify)
			close(c.watchRun)
			c.runCancel()
		}()

		for {
			select {
			case <-c.runCtx.Done():
				c.cancelRun("current query context done")
				return
			default:
				latency, err := c.getLatency()

				if err != nil {
					slog.Error(
						"query latency failed",
						slog.Any("error", err),
					)

					c.watchRun <- err
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

				var interval time.Duration
				if v := c.qryInterval.Load(); v != nil {
					interval = *v
				}

				// 一次性运行，直接退出
				if interval <= 0 {
					slog.Info("no interval specified, one time running.")
					return
				}

				select {
				case <-c.runCtx.Done():
					c.cancelRun("current query context done")
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

		c.reporterDone.Done()

		slog.Info("all reporters exitted")
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
		if interval > 0 {
			slog.Info(
				"starting latency client with interval",
				slog.Duration("interval", interval),
			)

			c.qryInterval.Store(&interval)
		} else {
			slog.Info("onetime running latency client")
		}

		c.notify = make(chan []*exFrontLatency, 10)

		if err = c.runQuerier(time.Second * 5); err != nil {
			return
		}

		go c.runReporter()
	})

	return
}

func (c *LatencyClient) Join() error {
	c.reporterDone.Wait()

	return <-c.watchRun
}

func (c *LatencyClient) cancelRun(msg string) {
	c.stopOnce.Do(func() {
		c.runCancel()

		slog.Info(msg)
	})
}

func (c *LatencyClient) Stop() {
	c.cancelRun("stopping current query&report runner")
}
