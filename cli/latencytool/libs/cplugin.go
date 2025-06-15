package libs

/*
#include <stdlib.h>
#include <strings.h>
#include <dlfcn.h>

const char* INIT_FUNC_NAME = "initialize";
const char* REPORT_FUNC_NAME = "report_fronts";
const char* DESTORY_FUNC_NAME = "destory";
const char* JOIN_FUNC_NAME = "join";
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"path"
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

	initFn    unsafe.Pointer
	reportFn  unsafe.Pointer
	destoryFn unsafe.Pointer
	joinFn    unsafe.Pointer
}

func (clib *CPluginLib) Init(ctx context.Context, cfgPath string) (err error) {
	clib.initOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		clib.ctx, clib.cancel = context.WithCancel(ctx)

		// TODO
		// if err = lib.initFn(lib.ctx, cfgPath); err != nil {
		// 	return
		// }

		clib.cfgPath = cfgPath
	})

	return
}

func (clib *CPluginLib) Stop() {
	// TODO
}

func (clib *CPluginLib) Join() error {
	// TODO
	return nil
}

func (clib *CPluginLib) ReportFronts(addrList ...string) error {
	// TODO
	return nil
}

func NewCPlugin(dir, name string) (lib *CPluginLib, err error) {
	var ext string
	switch runtime.GOOS {
	case "linux":
		ext = ".so"
	case "windows":
		ext = ".dll"
	default:
		return nil, errors.New("unsupported platform")
	}

	libPath := path.Join(
		dir, name, name+ext,
	)

	lib = &CPluginLib{
		libPath: libPath,
	}

	lib.loadOnce.Do(func() {
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
			lib.initFn = init
		}

		if report := C.dlsym(lib.plugin, C.REPORT_FUNC_NAME); report == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibFuncNotFound, C.GoString(msg),
			)
			return
		} else {
			lib.reportFn = report
		}

		if destory := C.dlsym(lib.plugin, C.DESTORY_FUNC_NAME); destory == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibFuncNotFound, C.GoString(msg),
			)
			return
		} else {
			lib.destoryFn = destory
		}

		if join := C.dlsym(lib.plugin, C.JOIN_FUNC_NAME); join == nil {
			msg := C.dlerror()

			err = fmt.Errorf(
				"%w: %s", errLibFuncNotFound, C.GoString(msg),
			)
			return
		} else {
			lib.joinFn = join
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
	})

	return
}
