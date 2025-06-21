package latency4go

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/olivere/elastic/v7"
	"github.com/valyala/bytebufferpool"
)

const (
	TIMERANGE_TERM  string = "captureTimestamp"
	USERS_TERM      string = "用户代码"
	TICK2ORDER_TERM string = "mdLatency"

	AGGREGATION_TERM    string = "exchangeAddr.keyword"
	AGGREGATION_FIELD   string = "交易所延迟"
	AGGREGATION_RESULTS string = "aggs_results"

	EXCHANGE_LATENCY_PERCENTS string = "exchange_latency_percents"
	EXCHANGE_LATENCY_EXTRA    string = "exchange_latency_extra"
	EXCHANGE_LATENCY_PRIORITY string = "exchange_latency_prority"
	DEFAULT_SORT              string = "params.mid"
)

type percentResults map[float64]float64

func (p percentResults) MarshalJSON() ([]byte, error) {
	data := make(map[string]float64, len(p))

	for k, v := range p {
		data[strconv.FormatFloat(k, 'f', 0, 64)] = v
	}

	return json.Marshal(data)
}

func (p percentResults) UnmarshalJSON(data []byte) error {
	values := make(map[string]float64)

	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	for k, v := range values {
		if key, err := strconv.ParseFloat(k, 64); err != nil {
			return err
		} else {
			p[key] = v
		}
	}

	return nil
}

func (p percentResults) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	keys := make([]float64, 0, len(p))
	for v := range p {
		keys = append(keys, v)
	}
	slices.Sort(keys)

	buff.WriteString("[")

	for idx, key := range keys {
		if idx > 0 {
			buff.WriteString(" ")
		}

		buff.WriteString(strconv.FormatFloat(key, 'f', 0, 64))
		buff.WriteString(":")
		buff.WriteString(strconv.FormatFloat(p[key], 'f', -1, 64))
	}
	buff.WriteString("]")

	return buff.String()
}

type ExFrontLatency struct {
	FrontAddr          string
	MaxLatency         float64
	MinLatency         float64
	AvgLatency         float64
	VarLatency         float64
	StdevLatency       float64
	SampleStdevLatency float64
	Percents           percentResults
	Priority           float64
	DocCount           int64
}

func (l *ExFrontLatency) UnmarshalJSON(data []byte) error {
	values := make(map[string]json.RawMessage)

	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	if err := json.Unmarshal(
		values["FrontAddr"], &l.FrontAddr,
	); err != nil {
		return err
	}

	if err := json.Unmarshal(
		values["MaxLatency"], &l.MaxLatency,
	); err != nil {
		return err
	}

	if err := json.Unmarshal(
		values["MinLatency"], &l.MinLatency,
	); err != nil {
		return err
	}

	if err := json.Unmarshal(
		values["AvgLatency"], &l.AvgLatency,
	); err != nil {
		return err
	}

	if err := json.Unmarshal(
		values["VarLatency"], &l.VarLatency,
	); err != nil {
		return err
	}

	if err := json.Unmarshal(
		values["StdevLatency"], &l.StdevLatency,
	); err != nil {
		return err
	}

	if err := json.Unmarshal(
		values["SampleStdevLatency"], &l.SampleStdevLatency,
	); err != nil {
		return err
	}

	if err := json.Unmarshal(
		values["Priority"], &l.Priority,
	); err != nil {
		return err
	}

	if err := json.Unmarshal(
		values["DocCount"], &l.DocCount,
	); err != nil {
		return err
	}

	l.Percents = make(percentResults)

	return json.Unmarshal(values["Percents"], &l.Percents)
}

func (l ExFrontLatency) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("FrontLantecy{FrontAddr:")
	buff.WriteString(l.FrontAddr)
	buff.WriteString(" Priority:")
	buff.WriteString(strconv.FormatFloat(l.Priority, 'f', -1, 64))
	buff.WriteString(" MaxLatency:")
	buff.WriteString(strconv.FormatFloat(l.MaxLatency, 'f', -1, 64))
	buff.WriteString(" MinLantecy:")
	buff.WriteString(strconv.FormatFloat(l.MinLatency, 'f', -1, 64))
	buff.WriteString(" AvgLatency:")
	buff.WriteString(strconv.FormatFloat(l.AvgLatency, 'f', -1, 64))
	buff.WriteString(" VarLatency:")
	buff.WriteString(strconv.FormatFloat(l.VarLatency, 'f', -1, 64))
	buff.WriteString(" StdevLatency:")
	buff.WriteString(strconv.FormatFloat(l.StdevLatency, 'f', -1, 64))
	buff.WriteString(" SampleStdevLatency:")
	buff.WriteString(strconv.FormatFloat(l.SampleStdevLatency, 'f', -1, 64))
	buff.WriteString(" Percents:")
	buff.WriteString(l.Percents.String())
	buff.WriteString(" DocCount:")
	buff.WriteString(strconv.FormatInt(l.DocCount, 10))
	buff.WriteString("}")

	return buff.String()
}

func ConvertSlice[S, D any](source []S, cvt func(S) D) []D {
	result := make([]D, len(source))

	for idx, v := range source {
		result[idx] = cvt(v)
	}

	return result
}

type timeRangeKey string

var (
	errInvalidTimeRangeArg = errors.New("invalid time range arg")
)

const TIMERANGE_KW_SPLIT = ","
const (
	TimeBefore     timeRangeKey = "before"
	TimeFrom       timeRangeKey = "from"
	TimeTo         timeRangeKey = "to"
	TimeBucket     timeRangeKey = "bucket"
	TimeBucketSize timeRangeKey = "size"
)

type TimeRange map[timeRangeKey]string
type Range [2]string

func (tr *TimeRange) Set(v string) error {
	if *tr == nil {
		*tr = make(TimeRange)
	}

	pairs := ConvertSlice(
		strings.Split(v, TIMERANGE_KW_SPLIT),
		func(v string) string {
			return strings.TrimSpace(v)
		},
	)

	for _, kv := range pairs {
		values := strings.SplitN(kv, "=", 2)

		if len(values) != 2 {
			// support for single before arg
			(*tr)[TimeBefore] = values[0]
		} else {
			key := timeRangeKey(values[0])
			value := values[1]

			switch key {
			case TimeBefore:
				// TODO: elastic duration string check
			case TimeFrom, TimeTo:
				v, err := time.ParseInLocation(
					"2006-01-02T15:04:05", value, time.Local,
				)

				if err != nil {
					return errors.Join(errInvalidTimeRangeArg, err)
				}

				value = v.Format(time.RFC3339)
			case TimeBucket, TimeBucketSize:

			default:
				return errInvalidTimeRangeArg
			}

			(*tr)[key] = value
		}
	}

	return nil
}

func (tr TimeRange) Type() string {
	return "TimeRange"
}

func (tr TimeRange) GetRange() (result Range) {
	before := tr[TimeBefore]
	from := tr[TimeFrom]
	to := tr[TimeTo]
	// bucket := tr[TimeBucket]
	// size := tr[TimeBucketSize]

	switch {
	case from != "" && to != "":
		result[0] = from
		result[1] = to

		return
	case from != "" && to == "":
		result[0] = from
		result[1] = "now"

		return
	case before != "" && from == "":
		result[0] = "now-" + before
		result[1] = "now"

		return
	default:
		slog.Error(
			"invalid or unsupported kwargs, fallback to before[5m]",
			slog.Any("kwargs", tr),
		)
	}

	result[0] = "now-5m"
	result[1] = "now"

	return
}

func (tr TimeRange) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	values := tr.GetRange()

	buff.WriteString("[")
	buff.WriteString(values[0])
	buff.WriteString(" ~ ")
	buff.WriteString(values[1])
	buff.WriteString("]")

	return buff.String()
}

type Tick2Order struct {
	From int
	To   int
}

func (to Tick2Order) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("[")
	buff.WriteString(strconv.FormatFloat(
		float64(to.From)/1000000.0, 'f', -1, 64,
	))
	buff.WriteString(" us ~ ")
	buff.WriteString(strconv.FormatFloat(
		float64(to.To)/1000000.0, 'f', -1, 64,
	))
	buff.WriteString(" us]")

	return buff.String()
}

type Users []string

func (u Users) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("[")
	for idx, v := range u {
		if idx > 0 {
			buff.WriteByte(' ')
		}

		buff.WriteString(v)
	}
	buff.WriteString("]")

	return buff.String()
}

type Quantile []float64

func (q Quantile) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("[")
	for idx, v := range q {
		if idx > 0 {
			buff.WriteByte(' ')
		}

		buff.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	}
	buff.WriteString("]")

	return buff.String()
}

type QueryConfig struct {
	Tick2Order Tick2Order
	TimeRange  TimeRange
	Users      Users

	DataSize int
	AggSize  int
	AggCount int
	Quantile Quantile

	SortBy string
}

var DefaultQueryConfig QueryConfig = QueryConfig{
	TimeRange: TimeRange{
		TimeBefore: "5m",
	},
}

func (cfg *QueryConfig) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("QueryConfig{TimeRange:")
	buff.WriteString(cfg.TimeRange.String())
	buff.WriteString(" Tick2Order:")
	buff.WriteString(cfg.Tick2Order.String())
	buff.WriteString(" UsersFilter:")
	buff.WriteString(cfg.Users.String())
	buff.WriteString(" DataSize:")
	buff.WriteString(strconv.Itoa(cfg.DataSize))
	buff.WriteString(" AggSize:")
	buff.WriteString(strconv.Itoa(cfg.AggSize))
	buff.WriteString(" AggCount:")
	buff.WriteString(strconv.Itoa(cfg.AggCount))
	buff.WriteString(" Quantiles:")
	buff.WriteString(fmt.Sprint(cfg.Quantile))
	buff.WriteString(" SortBy:'")
	buff.WriteString(cfg.SortBy)
	buff.WriteString("'}")

	return buff.String()
}

func (cfg *QueryConfig) makeQuery() (elastic.Query, elastic.Aggregation) {
	rangeValue := cfg.TimeRange.GetRange()

	var filters = []elastic.Query{
		elastic.NewRangeQuery(
			TIMERANGE_TERM,
		).Gte(
			rangeValue[0],
		).Lte(
			rangeValue[1],
		).Format("strict_date_optional_time"),
	}

	if len(cfg.Users) > 0 {
		filters = append(filters, elastic.NewTermsQuery(
			USERS_TERM,
			ConvertSlice(
				cfg.Users,
				func(v string) any { return v },
			)...,
		))
	}

	if cfg.Tick2Order.To != 0 {
		filters = append(filters, elastic.NewRangeQuery(
			TICK2ORDER_TERM,
		).Gte(cfg.Tick2Order.From).Lte(cfg.Tick2Order.To))
	}

	slog.Info(
		"data filter config",
		slog.String("qry_cfg", cfg.String()),
	)

	boolFilter := elastic.NewBoolQuery()
	boolFilter.Filter(filters...)

	var sortBy = DEFAULT_SORT
	if cfg.SortBy != "" {
		sortBy = cfg.SortBy
	}

	extAgg := elastic.NewExtendedStatsAggregation().Field(AGGREGATION_FIELD)
	percentileAgg := elastic.NewPercentilesAggregation().Field(
		AGGREGATION_FIELD,
	).Percentiles(cfg.Quantile...)
	priAgg := elastic.NewBucketScriptAggregation().BucketsPathsMap(
		map[string]string{
			"mid":          EXCHANGE_LATENCY_PERCENTS + ".50",
			"avg":          EXCHANGE_LATENCY_EXTRA + ".avg",
			"stdev":        EXCHANGE_LATENCY_EXTRA + ".std_deviation",
			"sample_stdev": EXCHANGE_LATENCY_EXTRA + ".std_deviation_sampling",
		},
	).Script(
		elastic.NewScript(sortBy),
	)

	// TODO: 时间窗口分组
	frontTerms := elastic.NewTermsAggregation().Field(
		AGGREGATION_TERM,
	).OrderByAggregation(
		EXCHANGE_LATENCY_PERCENTS+".50", true,
	).MinDocCount(
		cfg.AggCount,
	).SubAggregation(
		EXCHANGE_LATENCY_PERCENTS, percentileAgg,
	).SubAggregation(
		EXCHANGE_LATENCY_EXTRA, extAgg,
	).SubAggregation(
		EXCHANGE_LATENCY_PRIORITY, priAgg,
	)

	return boolFilter, frontTerms
}

func (cfg *QueryConfig) SetConfig(key, value string) error {
	switch strings.ToLower(key) {
	case "before", "range":
		cfg.TimeRange.Set(value)
	case "from":
		if v, err := strconv.Atoi(value); err != nil {
			return err
		} else {
			cfg.Tick2Order.From = v
		}
	case "to":
		if v, err := strconv.Atoi(value); err != nil {
			return err
		} else {
			cfg.Tick2Order.To = v
		}
	case "percents":
		values := []float64{}

		for _, v := range strings.Split(
			strings.TrimSuffix(
				strings.TrimPrefix(value, "["), "]",
			), ",",
		) {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return err
			}

			values = append(values, f)
		}

		cfg.Quantile = append(Quantile{}, values...)
	case "agg":
		if v, err := strconv.Atoi(value); err != nil {
			return err
		} else {
			cfg.AggSize = v
		}
	case "least":
		if v, err := strconv.Atoi(value); err != nil {
			return err
		} else {
			cfg.AggCount = v
		}
	case "user":
		cfg.Users = ConvertSlice(strings.Split(
			strings.TrimSuffix(
				strings.TrimPrefix(value, "["), "]",
			), ",",
		), func(v string) string {
			return strings.TrimSpace(v)
		})
	case "sort":
		cfg.SortBy = value
	default:
		return errors.New("unsupported config key")
	}

	return nil
}
