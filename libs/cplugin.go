package libs

/*
#cgo LDFLAGS: -ldl

#include <stdlib.h>
#include <strings.h>
#include <dlfcn.h>

const char* INIT_FUNC_NAME = "initialize";
const char* REPORT_FUNC_NAME = "report_fronts";
const char* DESTORY_FUNC_NAME = "destory";
const char* JOIN_FUNC_NAME = "join";

typedef int (*initialize)(char*);
typedef int (*report_fronts)(char**, int);
typedef int (*destory)();
typedef int (*join)();

int help_init(initialize fn, char* cfg_path) { return fn(cfg_path); }
int help_report_fronts(report_fronts fn, char** ptr, int len) { return fn(ptr, len); }
int help_destory(destory fn) { return fn(); }
int help_join(join fn) { return fn(); }
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"
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

	initFn    C.initialize
	reportFn  C.report_fronts
	destoryFn C.destory
	joinFn    C.join
}

func (clib *CPluginLib) Init(ctx context.Context, cfgPath string) (err error) {
	clib.initOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		clib.ctx, clib.cancel = context.WithCancel(ctx)

		cfg_path := C.CString(cfgPath)
		defer C.free(unsafe.Pointer(cfg_path))

		if rtn := C.help_init(clib.initFn, cfg_path); rtn != 0 {
			err = ErrInitFailed
			return
		}

		clib.cfgPath = cfgPath
	})

	return
}

func (clib *CPluginLib) Stop() {
	if rtn := C.help_destory(clib.destoryFn); rtn != 0 {
		slog.Error(
			"call module destory failed",
			slog.Int("rtn", int(rtn)),
		)
	}
}

func (clib *CPluginLib) Join() error {
	if rtn := C.help_join(clib.joinFn); rtn != 0 {
		return ErrJoinFailed
	}
	return nil
}

func (clib *CPluginLib) ReportFronts(addrList ...string) error {
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
		clib.reportFn, &arr[0], C.int(len(addrList)),
	); rtn != 0 {
		return ErrReportFailed
	}

	return nil
}

func NewCPlugin(
	dirName, libName string,
) (container *PluginContainer, err error) {
	var libPath string

	switch runtime.GOOS {
	case "linux":
		libDir := filepath.Join(dirName, libName)
		libPath = filepath.Join(libDir, libName+".so")

		if err := os.Setenv("LD_LIBRARY_PATH", libDir); err != nil {
			return nil, err
		}

		slog.Info(
			"lib environment setted for linux",
			slog.String("LD_LIBRARY_PATH", os.Getenv("LD_LIBRARY_PATH")),
		)
	case "windows":
		libPath = filepath.Join(
			dirName, libName, libName+".dll",
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
				plugin:     lib,
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
