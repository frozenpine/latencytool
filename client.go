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
const SLOG_TRADE = slog.LevelDebug - 1

const (
	ELASTIC_DOCUMENTS = "alldelaystatistics202*"
)

type Reporter func(*State) error

type LatencyReport struct {
	Timestamp time.Time
	Config    *QueryConfig
	Latency   []*ExFrontLatency
}

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
	reQuery     chan struct{}
	suspend     atomic.Bool
	suspendWg   sync.WaitGroup
	notify      chan *State

	sinkPath   string
	lastReport atomic.Pointer[LatencyReport]
	reporterWg sync.WaitGroup
	reporters  sync.Map
}

func (c *LatencyClient) Init(
	ctx context.Context,
	schema, host string, port int,
	sinkPath string, config *QueryConfig,
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

		if sinkPath != "" {
			var (
				sinkData []byte
				report   LatencyReport
			)
			if sinkData, err = os.ReadFile(sinkPath); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return
				}

				err = nil
				slog.Warn("sink file not exists, skip recover")
			} else if err = json.Unmarshal(
				sinkData, &report,
			); err != nil {
				slog.Error(
					"unmarshal sinked report failed",
					slog.Any("error", err),
				)
				err = nil
			} else {
				c.lastReport.Store(&report)
				slog.Info(
					"stored latency recovered from file",
					slog.String("sink_path", sinkPath),
					slog.Any("latency", c.lastReport.Load()),
				)
			}
		}

		c.client.Store(client)
		c.cfg.Store(config)
		c.sinkPath = sinkPath
		c.reQuery = make(chan struct{})
	})

	return
}

func (c *LatencyClient) GetAddr() string {
	return c.addr
}

func (c *LatencyClient) GetSinkPath() string {
	return c.sinkPath
}

func (c *LatencyClient) GetVersion() (string, error) {
	if ins := c.client.Load(); ins != nil {
		return ins.ElasticsearchVersion(c.addr)
	}

	return "", ErrNotInitialized
}

func (c *LatencyClient) queryLatency(cfg *QueryConfig) ([]*ExFrontLatency, error) {
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
		slog.Error(
			"query latency failed",
			slog.Any("error", err),
			slog.Any("rsp", rsp),
		)
		return nil, errors.Join(ErrReadResponse, err)
	}

	if rsp.Hits.TotalHits.Value > 0 {
		// print data record
		for _, hit := range rsp.Hits.Hits {
			if v, err := hit.Source.MarshalJSON(); err != nil {
				return nil, errors.Join(ErrReadResponse, err)
			} else {
				slog.Info(
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

	latencyList := []*ExFrontLatency{}

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

		latency := ExFrontLatency{
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

	slices.SortFunc(latencyList, func(l, r *ExFrontLatency) int {
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

func (c *LatencyClient) GetLastState() *State {
	if last := c.lastReport.Load(); last != nil {
		return NewState(
			last.Timestamp,
			c.cfg.Load().Clone(),
			last.Latency,
		)
	}

	return nil
}

func (c *LatencyClient) sinkLatency(
	ts time.Time, cfg *QueryConfig, latency []*ExFrontLatency,
) (rpt *LatencyReport) {
	defer func() {
		rpt = c.lastReport.Load()
	}()

	if len(latency) <= 0 {
		slog.Warn("empty latency result, skip sink")

		return nil
	}

	report := LatencyReport{
		Timestamp: ts,
		Config:    cfg.Clone(),
		Latency:   latency,
	}

	c.lastReport.Store(&report)

	if c.sinkPath == "" {
		return nil
	}

	if data, err := json.Marshal(report); err != nil {
		slog.Error(
			"marshal report data failed",
			slog.Any("error", err),
		)
	} else if err = os.WriteFile(c.sinkPath, data, os.ModePerm); err != nil {
		slog.Error(
			"sink report to file failed",
			slog.Any("error", err),
			slog.String("sink_file", c.sinkPath),
		)
	}

	return nil
}

func (c *LatencyClient) Suspend() bool {
	if c.suspend.CompareAndSwap(false, true) {
		c.suspendWg.Add(1)
		return true
	}

	return false
}

func (c *LatencyClient) Resume() bool {
	if c.suspend.CompareAndSwap(true, false) {
		c.suspendWg.Done()
		return true
	}

	return false
}

func (c *LatencyClient) ChangeInterval(interv time.Duration) time.Duration {
	if interv <= 0 {
		slog.Warn(
			"invalid query interval",
			slog.Duration("interval", interv),
		)
		return interv
	}

	old := *c.qryInterval.Swap(&interv)
	c.reQuery <- struct{}{}
	return old
}

func (c *LatencyClient) runQuerier(
	timeout time.Duration,
) error {
	c.runCtx, c.runCancel = context.WithCancel(c.ctx)
	c.watchRun = make(chan error, 1)

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
				c.suspendWg.Wait()
				if c.runCtx.Err() != nil {
					return
				}

				currCfg := *c.cfg.Load()
				ts := time.Now()
				latency, err := c.queryLatency(&currCfg)

				if err != nil {
					slog.Error(
						"query latency failed",
						slog.Any("error", err),
					)

					// 一次性运行直接退出
					if *c.qryInterval.Load() <= 0 {
						c.watchRun <- err
						return
					}

					continue
				}

				var state *State

				if report := c.sinkLatency(ts, &currCfg, latency); report != nil {
					state = NewState(
						report.Timestamp, report.Config, report.Latency,
					)
				} else {
					slog.Warn("no valid report stored, use query config")
					state = NewState(ts, &currCfg, latency)
				}

				select {
				case c.notify <- state:
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
					c.cancelRun("no interval specified, one time running.")
					return
				}

				select {
				case <-c.runCtx.Done():
					c.cancelRun("current query context done")
					return
				case <-c.reQuery:
					slog.Info(
						"interval or config changed",
						slog.Duration("interval", *c.qryInterval.Load()),
						slog.Any("config", c.cfg.Load()),
					)
				case <-time.After(interval):
				}
			}
		}
	}()

	return nil
}

func (c *LatencyClient) runReporter() {
	// self log reporter
	c.reporterWg.Add(1)

	defer func() {
		c.reporters.Range(func(key, value any) bool {
			c.reporterWg.Done()
			return true
		})

		c.reporterWg.Done()

		slog.Info("all reporters exitted")
	}()

	for state := range c.notify {
		slog.Info(
			"tool log reporter for priorities",
			slog.Any("addr", state.AddrList),
		)

		c.reporters.Range(func(key, value any) bool {
			name, ok := key.(string)

			if !ok {
				c.reporters.Delete(key)
				c.reporterWg.Done()
				return true
			}

			reportFn, ok := value.(Reporter)
			if !ok {
				c.reporters.Delete(key)
				c.reporterWg.Done()

				return true
			}

			slog.Info(
				"sending latency state to reporter",
				slog.String("reporter", name),
			)

			if err := reportFn(state); err != nil {
				slog.Error(
					"send latency state to reporter failed",
					slog.Any("error", err),
					slog.String("reporter", name),
				)
			}

			return true
		})
	}

	slog.Info("latency notify channel closed")
}

func (c *LatencyClient) AddReporter(name string, reporter Reporter) error {
	if name == "" || reporter == nil {
		return ErrInvalidReporter
	}

	if exist, loaded := c.reporters.LoadOrStore(name, reporter); loaded {
		slog.Warn(
			"reporter with name already exist",
			slog.String("name", name),
			slog.Any("reporter", exist),
		)
		return ErrInvalidReporter
	}

	c.reporterWg.Add(1)

	return nil
}

func (c *LatencyClient) DelReporter(name string) error {
	if name == "" {
		return ErrInvalidReporter
	}

	if reporter, exists := c.reporters.LoadAndDelete(
		name,
	); !exists || reporter == nil {
		return fmt.Errorf(
			"%w: %s reporter not exists", ErrInvalidReporter, name,
		)
	}

	return nil
}

func (c *LatencyClient) SetConfig(data map[string]string) error {
	if len(data) <= 0 {
		return errors.New("empty config data")
	}

	tmpCfg := *c.cfg.Load()
	if cfg, exists := data["config"]; exists {
		if err := json.Unmarshal([]byte(cfg), &tmpCfg); err != nil {
			return err
		}
	} else {
		for k, v := range data {
			if err := tmpCfg.SetConfig(k, v); err != nil {
				return err
			}
		}
	}

	c.cfg.Store(&tmpCfg)
	c.reQuery <- struct{}{}

	return nil
}

func (c *LatencyClient) GetConfig() *QueryConfig {
	return c.cfg.Load().Clone()
}

func (c *LatencyClient) GetInterval() time.Duration {
	return *c.qryInterval.Load()
}

func (c *LatencyClient) QueryLatency(kwargs map[string]string) (*State, error) {
	var tmpCfg QueryConfig = *c.cfg.Load().Clone()

	if cfg, exists := kwargs["config"]; exists {
		if err := json.Unmarshal([]byte(cfg), &tmpCfg); err != nil {
			return nil, err
		}
	} else {
		for k, v := range kwargs {
			if err := tmpCfg.SetConfig(k, v); err != nil {
				return nil, err
			}
		}
	}

	ts := time.Now()
	latency, err := c.queryLatency(&tmpCfg)
	if err != nil {
		return nil, err
	}

	return NewState(ts, &tmpCfg, latency), nil
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

		c.notify = make(chan *State, 10)

		if err = c.runQuerier(time.Second * 5); err != nil {
			return
		}

		go c.runReporter()
	})

	return
}

func (c *LatencyClient) Join() error {
	c.reporterWg.Wait()

	return <-c.watchRun
}

func (c *LatencyClient) cancelRun(msg string) {
	c.stopOnce.Do(func() {
		c.runCancel()
		c.Resume()

		slog.Info(msg)
	})
}

func (c *LatencyClient) Stop() {
	c.cancelRun("stopping current query&report runner")
}
