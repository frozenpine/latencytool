package libs

/*
#cgo LDFLAGS: -ldl

#include <stdlib.h>
#include <strings.h>
#include <dlfcn.h>

const char* INIT_FUNC_NAME = "initialize";
const char* REPORT_FUNC_NAME = "report_fronts";
const char* SEATS_FUNC_NAME = "seats";
const char* PRIORITY_FUNC_NAME = "priority";
const char* DESTORY_FUNC_NAME = "destory";
const char* JOIN_FUNC_NAME = "join";

typedef struct _seat {
	int idx;
	char* addr;
} seat_t;

typedef struct _level {
	int* levels;
	int len;
} level_t;

typedef int (*initialize)(char*);
typedef int (*report_fronts)(char**, int);
typedef int (*seats)(seat_t**);
typedef int (*priority)(level_t**);
typedef int (*destory)();
typedef int (*join)();

int help_init(initialize fn, char* cfg_path) { return fn(cfg_path); }
int help_report_fronts(report_fronts fn, char** ptr, int len) { return fn(ptr, len); }
int help_destory(destory fn) { return fn(); }
int help_join(join fn) { return fn(); }
int help_seats(seats fn, seat_t** ptr) { return fn(ptr); }
int help_priority(priority fn, level_t** ptr) { return fn(ptr); }
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/frozenpine/latency4go"
)

type CPluginLib struct {
	libPath string
	plugin  unsafe.Pointer

	loadOnce   sync.Once
	unloadOnce sync.Once
	initOnce   sync.Once

	ctx     context.Context
	cancel  context.CancelFunc
	cfgPath string

	initFn     C.initialize
	reportFn   C.report_fronts
	seatsFn    C.seats
	priorityFn C.priority
	destoryFn  C.destory
	joinFn     C.join
}

func (cLib *CPluginLib) Init(ctx context.Context, cfgPath string) (err error) {
	cLib.initOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		cLib.ctx, cLib.cancel = context.WithCancel(ctx)

		cfg_path := C.CString(cfgPath)
		defer C.free(unsafe.Pointer(cfg_path))

		if rtn := C.help_init(cLib.initFn, cfg_path); rtn != 0 {
			err = ErrInitFailed
			return
		}

		cLib.cfgPath = cfgPath
	})

	return
}

func (cLib *CPluginLib) Stop() {
	if rtn := C.help_destory(cLib.destoryFn); rtn != 0 {
		slog.Error(
			"call module destory failed",
			slog.Int("rtn", int(rtn)),
		)
	}
}

func (cLib *CPluginLib) Join() error {
	if rtn := C.help_join(cLib.joinFn); rtn != 0 {
		return ErrJoinFailed
	}
	return nil
}

func (cLib *CPluginLib) ReportFronts(addrList ...string) error {
	arr := make([]*C.char, 0, len(addrList))

	for _, addr := range addrList {
		arr = append(arr, C.CString(addr))
	}

	defer func() {
		for _, v := range arr {
			C.free(unsafe.Pointer(v))
		}
	}()

	if rtn := C.help_report_fronts(
		cLib.reportFn, &arr[0], C.int(len(addrList)),
	); rtn != 0 {
		return ErrReportFailed
	}

	return nil
}

func (cLib *CPluginLib) Seats() []Seat {
	buff := make([]*C.seat_t, 15)

	count := C.help_seats(
		cLib.seatsFn, &buff[0],
	)

	return latency4go.ConvertSlice(
		buff[:int(count)], func(v *C.seat_t) Seat {
			addr := C.GoString(v.addr)
			C.free(unsafe.Pointer(v.addr))
			return Seat{
				Idx:  int(v.idx),
				Addr: addr,
			}
		},
	)
}

func (cLib *CPluginLib) Priority() [][]int {
	buff := make([]*C.level_t, 15)

	count := C.help_priority(
		cLib.priorityFn, &buff[0],
	)

	return latency4go.ConvertSlice(
		buff[:int(count)], func(v *C.level_t) []int {
			lvl := make([]int, int(v.len))

			sli := *(*[]int)(unsafe.Pointer(&reflect.SliceHeader{
				Data: uintptr(unsafe.Pointer(v.levels)),
				Len:  int(v.len),
				Cap:  int(v.len),
			}))

			copy(lvl, sli)

			return lvl
		},
	)
}

func NewCPlugin(
	dirName, libName string,
) (container *PluginContainer, err error) {
	var libPath string

	libIdentiy := strings.SplitN(libName, ".", 2)

	switch runtime.GOOS {
	case "linux":
		libDir := filepath.Join(dirName, libIdentiy[0])
		libPath = filepath.Join(libDir, libIdentiy[0]+".so")

		if err := os.Setenv("LD_LIBRARY_PATH", libDir); err != nil {
			return nil, err
		}

		slog.Info(
			"lib environment setted for linux",
			slog.String("LD_LIBRARY_PATH", os.Getenv("LD_LIBRARY_PATH")),
		)
	case "windows":
		libPath = filepath.Join(
			dirName, libIdentiy[0], libIdentiy[0]+".dll",
		)
	default:
		return nil, errors.New("unsupported platform")
	}

	lib := &CPluginLib{
		libPath: libPath,
	}

	lib.loadOnce.Do(func() {
		if _, loaded := registeredPlugins.LoadOrStore(
			libName, &PluginContainer{
				pluginType: CPlugin,
				libDir:     dirName,
				name:       libName,
				Plugin:     lib,
			},
		); loaded {
			err = fmt.Errorf(
				"%w: plugin already loaded", errLibOpenFailed,
			)

			return
		}

		lib_path := C.CString(lib.libPath)
		defer C.free(unsafe.Pointer(lib_path))

		lib.plugin = C.dlopen(lib_path, C.RTLD_LAZY)
		if lib.plugin == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibOpenFailed, C.GoString(msg),
			)
			return
		}

		if init := C.dlsym(lib.plugin, C.INIT_FUNC_NAME); init == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibFuncNotFound, C.GoString(msg),
			)
			return
		} else {
			lib.initFn = (C.initialize)(init)
		}

		if report := C.dlsym(lib.plugin, C.REPORT_FUNC_NAME); report == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibFuncNotFound, C.GoString(msg),
			)
			return
		} else {
			lib.reportFn = (C.report_fronts)(report)
		}

		if seat := C.dlsym(lib.plugin, C.SEATS_FUNC_NAME); seat == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibFuncNotFound, C.GoString(msg),
			)
			return
		} else {
			lib.seatsFn = (C.seats)(seat)
		}

		if pri := C.dlsym(lib.plugin, C.PRIORITY_FUNC_NAME); pri == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibFuncNotFound, C.GoString(msg),
			)
			return
		} else {
			lib.priorityFn = (C.priority)(pri)
		}

		if destory := C.dlsym(lib.plugin, C.DESTORY_FUNC_NAME); destory == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibFuncNotFound, C.GoString(msg),
			)
			return
		} else {
			lib.destoryFn = (C.destory)(destory)
		}

		if join := C.dlsym(lib.plugin, C.JOIN_FUNC_NAME); join == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibFuncNotFound, C.GoString(msg),
			)
			return
		} else {
			lib.joinFn = (C.join)(join)
		}

		runtime.SetFinalizer(lib, func(plugin *CPluginLib) {
			plugin.cancel()

			plugin.unloadOnce.Do(func() {
				C.dlclose(plugin.plugin)
				plugin.plugin = nil
				plugin.initFn = nil
				plugin.reportFn = nil
				plugin.destoryFn = nil
				plugin.joinFn = nil
			})
		})

		container, err = GetRegisteredPlugin(libName)
	})

	return
}
