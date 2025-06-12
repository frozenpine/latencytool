package latency4go

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	"github.com/olivere/elastic/v7"
	"github.com/valyala/bytebufferpool"
)

const (
	TIMERANGE_TERM            string = "captureTimestamp"
	USERS_TERM                string = "用户代码"
	TICK2ORDER_TERM           string = "mdLatency"
	EXCHANGE_LATENCY_MAX      string = "exchange_latency_max"
	EXCHANGE_LATENCY_MIN      string = "exchange_latency_min"
	EXCHANGE_LATENCY_PERCENTS string = "exchange_latency_percents"
	AGGREGATION_TERM          string = "exchangeAddr.keyword"
	AGGREGATION_FIELD         string = "交易所延迟"
	AGGREGATION_RESULTS       string = "aggs_results"
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

type exFrontLatency struct {
	FrontAddr  string
	MaxLatency float64
	MinLatency float64
	Percents   percentResults
}

func (l *exFrontLatency) UnmarshalJSON(data []byte) error {
	values := make(map[string]json.RawMessage)

	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	if err := json.Unmarshal(values["FrontAddr"], &l.FrontAddr); err != nil {
		return err
	}

	if err := json.Unmarshal(values["MaxLatency"], &l.MaxLatency); err != nil {
		return err
	}

	if err := json.Unmarshal(values["MinLatency"], &l.MinLatency); err != nil {
		return err
	}

	l.Percents = make(percentResults)

	return json.Unmarshal(values["Percents"], &l.Percents)
}

func (l exFrontLatency) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("FrontLantecy{FrontAddr:")
	buff.WriteString(l.FrontAddr)
	buff.WriteString(" MaxLatency:")
	buff.WriteString(strconv.FormatFloat(l.MaxLatency, 'f', -1, 64))
	buff.WriteString(" MinLantecy:")
	buff.WriteString(strconv.FormatFloat(l.MinLatency, 'f', -1, 64))
	buff.WriteString(" Percents:")
	buff.WriteString(l.Percents.String())
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

type TimeRange struct {
	From string
	To   string
}

func (tr TimeRange) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("TimeRange{")
	buff.WriteString(tr.From)
	buff.WriteString(" ~ ")
	buff.WriteString(tr.To)
	buff.WriteString("}")

	return buff.String()
}

type Tick2Order struct {
	From int
	To   int
}

func (to Tick2Order) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("Tick2OrderRange{")
	buff.WriteString(strconv.Itoa(to.From / 1000000))
	buff.WriteString(" us ~ ")
	buff.WriteString(strconv.Itoa(to.To / 1000000))
	buff.WriteString(" us}")

	return buff.String()
}

type Users []string

type Quantile []float64

type QueryConfig struct {
	Tick2Order Tick2Order
	TimeRange  TimeRange
	Users      Users

	DataSize int
	AggSize  int
	Quantile Quantile
}

func (cfg *QueryConfig) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("QueryConfig{")
	buff.WriteString(cfg.TimeRange.String())
	buff.WriteByte(' ')
	buff.WriteString(cfg.Tick2Order.String())
	buff.WriteString(" UsersFilter:")
	buff.WriteString(fmt.Sprint(cfg.Users))
	buff.WriteString(" DataSize:")
	buff.WriteString(strconv.Itoa(cfg.DataSize))
	buff.WriteString(" AggSize:")
	buff.WriteString(strconv.Itoa(cfg.AggSize))
	buff.WriteString(" Quantiles:")
	buff.WriteString(fmt.Sprint(cfg.Quantile))
	buff.WriteString("}")

	return buff.String()
}

func (cfg *QueryConfig) makeQuery() (elastic.Query, elastic.Aggregation) {
	timeRange := []string{"now-1d/d", cfg.TimeRange.To}

	if cfg.TimeRange.From != "" {
		timeRange[0] = fmt.Sprintf("now-%s", cfg.TimeRange.From)
	}

	var filters = []elastic.Query{
		elastic.NewRangeQuery(
			TIMERANGE_TERM,
		).Gte(
			timeRange[0],
		).Lte(
			timeRange[1],
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

	maxAgg := elastic.NewMaxAggregation().Field(AGGREGATION_FIELD)
	minAgg := elastic.NewMinAggregation().Field(AGGREGATION_FIELD)
	percentileAgg := elastic.NewPercentilesAggregation().Field(
		AGGREGATION_FIELD,
	).Percentiles(cfg.Quantile...)

	terms := elastic.NewTermsAggregation().Field(
		AGGREGATION_TERM,
	).OrderByAggregation(
		EXCHANGE_LATENCY_PERCENTS+".50", true,
	).SubAggregation(
		EXCHANGE_LATENCY_PERCENTS, percentileAgg,
	).SubAggregation(
		EXCHANGE_LATENCY_MIN, minAgg,
	).SubAggregation(
		EXCHANGE_LATENCY_MAX, maxAgg,
	)

	return boolFilter, terms
}
