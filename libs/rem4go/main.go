package main

//#cgo LDFLAGS: -L.
import "C"

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sync"
	"unsafe"

	"github.com/frozenpine/latency4go/libs"
	"github.com/pelletier/go-toml/v2"
	"gitlab.devops.rdrk.com.cn/quant/rem4go/emc"
)

var (
	version, goVersion, gitVersion, buildTime string

	api       = emc.EmcApi{}
	cfg       = emc.Config{}
	apiCtx    context.Context
	apiCancel context.CancelFunc
	initOnce  = sync.Once{}
	stopOnce  = sync.Once{}
)

func init() {
	fmt.Printf(
		"[LIB emc4go] %s, Commit: %s, Build: %s@%s",
		version, gitVersion, buildTime, goVersion,
	)
}

func main() {}

func Init(ctx context.Context, cfgPath string) (err error) {
	initOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		cfgFile, failed := os.OpenFile(cfgPath, os.O_RDONLY, os.ModePerm)
		if failed != nil {
			err = errors.Join(libs.ErrInitFailed, failed)
			return
		}

		decoder := toml.NewDecoder(cfgFile)

		if failed := decoder.Decode(&map[string]any{
			"emc": &cfg,
		}); failed != nil {
			err = errors.Join(libs.ErrInitFailed, failed)
			return
		}

		apiCtx, apiCancel = context.WithCancel(ctx)

		if err = api.Init(apiCtx, &cfg); err != nil {
			return
		}

		if err = api.Connect(); err != nil {
			return
		}

		if err = api.Login(); err != nil {
			return
		}
	})

	return
}

//export initialize
func initialize(cfgPath *C.char) C.int {
	emcCfg := C.GoString(cfgPath)

	if err := Init(context.Background(), emcCfg); err != nil {
		slog.Error(
			"Rem module initialize failed",
			slog.Any("error", err),
		)

		return -1
	}

	return 0
}

func ReportFronts(addrList ...string) error {
	if req, err := api.MakeSeatsPriority(addrList...); err != nil {
		return errors.Join(libs.ErrReportFailed, err)
	} else if err = api.SendMktSessPriChange(req); err != nil {
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
