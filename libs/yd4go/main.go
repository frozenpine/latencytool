package main

//#cgo LDFLAGS: -L.
import "C"

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"unsafe"

	"github.com/frozenpine/latency4go/libs"
	"gitlab.devops.rdrk.com.cn/quant/yd4go"
)

var (
	version, goVersion, gitVersion, buildTime string

	api       = yd4go.YdApi{}
	apiCtx    context.Context
	apiCancel context.CancelFunc
	initOnce  = sync.Once{}
	stopOnce  = sync.Once{}
)

func init() {
	fmt.Printf(
		"[LIB emc4go] %s, Commit: %s, Build: %s@%s, YdApiVersion: %s",
		version, gitVersion, buildTime, goVersion, yd4go.GetApiVersion(),
	)
}

func main() {}

func Init(ctx context.Context, cfgPath string) (err error) {
	initOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		apiCtx, apiCancel = context.WithCancel(ctx)

		if !api.Init(apiCtx, cfgPath) {
			err = fmt.Errorf(
				"%w: Yd call api Init failed", libs.ErrInitFailed,
			)
		}

		if !api.Login("", "", "", "") {
			err = errors.New("request login failed")
		}
	})

	return
}

//export initialize
func initialize(cfgPath *C.char) C.int {
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
	if err := api.SelectConnections(addrs...); err != nil {
		return errors.Join(libs.ErrReportFailed, err)
	}

	return nil
}

//export report_fronts
func report_fronts(ptr **C.char, len C.int) C.int {
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
	apiCancel()

	return 0
}

func Join() error {
	<-api.Join()

	return nil
}

//export join
func join() C.int {
	Join()

	return 0
}
