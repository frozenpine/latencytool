module github.com/frozenpine/latency4go

go 1.24.3

require (
	github.com/frozenpine/msgqueue v0.0.3
	github.com/olivere/elastic/v7 v7.0.32
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/spf13/cobra v1.9.1
	github.com/valyala/bytebufferpool v1.0.0
	gitlab.devops.rdrk.com.cn/quant/rem4go v0.0.0-20250612024936-8707660650a5
	gitlab.devops.rdrk.com.cn/quant/yd4go v0.0.0-20250612024858-18b1ed61721b
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require (
	github.com/frozenpine/pool v0.0.14 // indirect
	github.com/gocarina/gocsv v0.0.0-20240520201108-78e41c74b4b1 // indirect
	github.com/gofrs/uuid v4.3.1+incompatible // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
)

replace gitlab.devops.rdrk.com.cn/quant/yd4go v0.0.0-20250612024858-18b1ed61721b => github.com/frozenpine/yd4go v0.0.0-20250612024858-18b1ed61721b

replace gitlab.devops.rdrk.com.cn/quant/rem4go v0.0.0-20250612024936-8707660650a5 => github.com/frozenpine/rem4go v0.0.0-20250612024936-8707660650a5
