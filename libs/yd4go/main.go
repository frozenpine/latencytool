package main

//#cgo LDFLAGS: -L.
import "C"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/frozenpine/latency4go"
	"github.com/frozenpine/latency4go/libs"
	"gitlab.devops.rdrk.com.cn/quant/yd4go"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	version, goVersion, gitVersion, buildTime string

	api       atomic.Pointer[yd4go.YdApi]
	apiCtx    context.Context
	apiCancel context.CancelFunc
	initOnce  = sync.Once{}
	stopOnce  = sync.Once{}
)

func init() {
	fmt.Printf(
		"[LIB yd4go] %s, Commit: %s, Build: %s@%s, YdApiVersion: %s",
		version, gitVersion, buildTime, goVersion, yd4go.GetApiVersion(),
	)
}

func main() {}

func SetLogger(lvl slog.Level, logFile string, logSize, logKeep int) error {
	var (
		addSource bool
		logWr     io.Writer
	)

	if logFile != "" {
		logWr = &lumberjack.Logger{
			Filename: logFile,
			MaxSize:  logSize,
			MaxAge:   logKeep,
			Compress: true,
		}

		logWr = io.MultiWriter(logWr, os.Stderr)
	} else {
		logWr = os.Stderr
	}

	if lvl <= slog.LevelDebug {
		addSource = true
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(
		logWr, &slog.HandlerOptions{
			AddSource: addSource,
			Level:     lvl,
		},
	)))

	slog.Debug("yd4go plugin logger initiated")

	return nil
}

//export set_logger
func set_logger(lvl C.int, logFile *C.char, logSize, logKeep C.int) C.int {
	if err := SetLogger(
		slog.Level(lvl),
		C.GoString(logFile),
		int(logSize), int(logKeep),
	); err != nil {
		fmt.Fprintf(
			os.Stderr, "yd4go plugin set logger failed: %+v", err,
		)
		return -1
	} else {
		return 0
	}
}

func Init(ctx context.Context, cfgPath string) (err error) {
	initOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		if !api.CompareAndSwap(nil, &yd4go.YdApi{}) {
			err = fmt.Errorf(
				"%w: Yd api already exists: %+v", api.Load(),
			)
			return
		}

		apiCtx, apiCancel = context.WithCancel(ctx)

		slog.Info(
			"initializing ydapi",
			slog.String("config", cfgPath),
		)
		if !api.Load().Init(apiCtx, cfgPath) {
			err = fmt.Errorf(
				"%w: Yd call api Init failed", libs.ErrInitFailed,
			)
			return
		}

		slog.Info("login to yd with config authentications")
		if !api.Load().Login("", "", "", "") {
			err = errors.New("request login failed")
			return
		}

		slog.Info("yd4go plugin intialized")
	})

	return
}

//export initialize
func initialize(cfgPath *C.char) C.int {
	slog.Debug("yd4go c bridge [initialize] function called")
	ydCfg := C.GoString(cfgPath)

	if err := Init(context.Background(), ydCfg); err != nil {
		slog.Error(
			"Yd module initialize failed",
			slog.Any("error", err),
		)
		return -1
	}

	return 0
}

func ReportFronts(addrs ...string) error {
	slog.Info(
		"reporting addr list to yd",
		slog.Any("addr_list", addrs),
	)
	if err := api.Load().SelectConnections(addrs...); err != nil {
		return errors.Join(libs.ErrReportFailed, err)
	}

	return nil
}

//export report_fronts
func report_fronts(ptr **C.char, len C.int) C.int {
	slog.Debug("yd4go c bridge [report_fronts] function called")
	count := int(len)

	addrs := make([]string, 0, count)
	ptrArr := *(*[]*C.char)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(ptr)),
		Len:  count,
		Cap:  count,
	}))

	for _, cStr := range ptrArr {
		addrs = append(addrs, C.GoString(cStr))
	}

	if err := ReportFronts(addrs...); err != nil {
		slog.Error(
			"Yd module report fronts failed",
			slog.Any("error", err),
		)
		return -2
	}

	return 0
}

//export destory
func destory() C.int {
	slog.Debug("yd4go c bridge [destory] function called")
	apiCancel()

	return 0
}

func Join() error {
	<-api.Load().Join()

	return nil
}

//export join
func join() C.int {
	slog.Debug("yd4go c bridge [join] function called")
	Join()

	return 0
}

func Seats() []libs.Seat {
	seats := api.Load().Seats()

	return latency4go.ConvertSlice(
		seats, func(v struct {
			Idx  int
			Addr string
		}) libs.Seat {
			return libs.Seat{
				Idx:  v.Idx,
				Addr: v.Addr,
			}
		},
	)
}

//export seats
func seats(buff unsafe.Pointer) C.int {
	slog.Debug("yd4go c bridge [seats] function called")
	seats := Seats()

	buffSlice := *(*[]*struct {
		idx  C.int
		addr *C.char
	})(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(buff),
		Len:  len(seats),
		Cap:  len(seats),
	}))

	for idx, s := range seats {
		buffSlice[idx].idx = C.int(s.Idx)
		buffSlice[idx].addr = C.CString(s.Addr)
	}

	return C.int(len(seats))
}

func Priority() [][]int {
	slog.Debug("yd4go c bridge [priority] function called")
	prio := api.Load().Priority()

	result := [][]int{}

	for _, v := range prio {
		result = append(result, v)
	}

	return result
}

//export priority
func priority(buff unsafe.Pointer) C.int {
	prio := Priority()

	buffSlice := *(*[]struct {
		levels *C.int
		len    C.int
	})(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(buff),
		Len:  len(prio),
		Cap:  len(prio),
	}))

	for idx, lvl := range prio {
		buffSlice[idx] = struct {
			levels *C.int
			len    C.int
		}{
			levels: (*C.int)(unsafe.Pointer(&lvl[0])),
			len:    C.int(len(lvl)),
		}
	}

	return C.int(len(prio))
}
