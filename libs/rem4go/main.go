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
	"slices"
	"sync"
	"unsafe"

	"github.com/frozenpine/latency4go"
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
		"[LIB emc4go] %s, Commit: %s, Build: %s@%s, EmcApiVersion: %s",
		version, gitVersion, buildTime, goVersion, emc.GetApiVersion(),
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

func Seats() []libs.Seat {
	seats := api.Seats()

	return latency4go.ConvertSlice(
		seats, func(v emc.Seat) libs.Seat {
			return libs.Seat{
				Idx:  v.Index,
				Addr: v.Address,
			}
		},
	)
}

//export seats
func seats(buff unsafe.Pointer) C.int {
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
	prio := api.Priority()

	results := [][]int{}

	for _, p := range prio.Levels {
		if p[0] == 0 {
			break
		}

		results = append(results, p[0:slices.Index(p[:], 0)])
	}

	return results
}

//export priority
func priority(buff unsafe.Pointer) C.int {
	prio := Priority()

	buffSlice := *(*[]*struct {
		levels *C.int
		len    C.int
	})(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(buff),
		Len:  len(prio),
		Cap:  len(prio),
	}))

	for idx, lvl := range prio {
		buffSlice[idx] = &struct {
			levels *C.int
			len    C.int
		}{
			levels: (*C.int)(unsafe.Pointer(&lvl[0])),
			len:    C.int(len(lvl)),
		}
	}

	return C.int(len(prio))
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
