module github.com/frozenpine/latency4go

go 1.24.3

require (
	github.com/frozenpine/msgqueue v0.0.3
	github.com/james-barrow/golang-ipc v1.2.4
	github.com/olivere/elastic/v7 v7.0.32
	github.com/pelletier/go-toml/v2 v2.2.4
	github.com/spf13/cobra v1.9.1
	github.com/valyala/bytebufferpool v1.0.0
	gitlab.devops.rdrk.com.cn/quant/rem4go v0.0.0-20250612024936-8707660650a5
	gitlab.devops.rdrk.com.cn/quant/yd4go v0.0.0-20250612024858-18b1ed61721b
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/frozenpine/pool v0.0.14 // indirect
	github.com/gdamore/encoding v1.0.0 // indirect
	github.com/gdamore/tcell/v2 v2.7.1 // indirect
	github.com/gocarina/gocsv v0.0.0-20240520201108-78e41c74b4b1 // indirect
	github.com/gofrs/uuid v4.3.1+incompatible // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/tview v0.0.0-20250501113434-0c592cd31026 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/term v0.17.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
)

replace gitlab.devops.rdrk.com.cn/quant/yd4go v0.0.0-20250612024858-18b1ed61721b => github.com/frozenpine/yd4go v0.0.0-20250612024858-18b1ed61721b

replace gitlab.devops.rdrk.com.cn/quant/rem4go v0.0.0-20250612024936-8707660650a5 => github.com/frozenpine/rem4go v0.0.0-20250612024936-8707660650a5
