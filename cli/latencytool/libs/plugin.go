package libs

import (
	"context"
	"errors"
	"fmt"
	"path"
	"plugin"
	"runtime"
	"sync"
)

const (
	INIT_FUNC_NAME   = "CreateInstance"
	REPORT_FUNC_NAME = "ReportFronts"
	JOIN_FUNC_NAME   = "Join"
)

var (
	errLibOpenFailed   = errors.New("open lib failed")
	errLibFuncNotFound = errors.New("lib func not found")
)

type Plugin interface {
	Init(context.Context, string) error
	Stop()
	Join() error
	ReportFronts(...string) error
}

type PluginLib struct {
	libPath string
	plugin  *plugin.Plugin

	loadOnce   sync.Once
	unloadOnce sync.Once
	initOnce   sync.Once

	ctx     context.Context
	cancel  context.CancelFunc
	cfgPath string

	initFn   func(context.Context, string) error
	reportFn func(...string) error
	joinFn   func() error
}

func (lib *PluginLib) Init(ctx context.Context, cfgPath string) (err error) {
	lib.initOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		lib.ctx, lib.cancel = context.WithCancel(ctx)

		if err = lib.initFn(lib.ctx, cfgPath); err != nil {
			return
		}

		lib.cfgPath = cfgPath
	})

	return
}

func (lib *PluginLib) ReportFronts(addrList ...string) error {
	return lib.reportFn(addrList...)
}

func (lib *PluginLib) Stop() {
	lib.cancel()
}

func (lib *PluginLib) Join() error {
	return lib.joinFn()
}

func NewPlugin(dir, name string) (lib *PluginLib, err error) {
	libPath := path.Join(
		dir, name, name+".plugin",
	)

	lib = &PluginLib{
		libPath: libPath,
	}

	lib.loadOnce.Do(func() {
		if lib.plugin, err = plugin.Open(lib.libPath); err != nil {
			err = errors.Join(errLibOpenFailed, err)
			return
		}

		if init, failed := lib.plugin.Lookup(INIT_FUNC_NAME); failed != nil {
			err = errors.Join(errLibFuncNotFound, failed)
			return
		} else if initFn, ok := init.(func(context.Context, string) error); !ok {
			err = fmt.Errorf(
				"%w: %s not found", errLibFuncNotFound, INIT_FUNC_NAME,
			)
			return
		} else {
			lib.initFn = initFn
		}

		if report, failed := lib.plugin.Lookup(REPORT_FUNC_NAME); failed != nil {
			err = errors.Join(errLibFuncNotFound, err)
			return
		} else if reportFn, ok := report.(func(...string) error); !ok {
			err = fmt.Errorf(
				"%w: %s not found", errLibFuncNotFound, REPORT_FUNC_NAME,
			)
			return
		} else {
			lib.reportFn = reportFn
		}

		if join, failed := lib.plugin.Lookup(JOIN_FUNC_NAME); failed != nil {
			err = errors.Join(errLibFuncNotFound, failed)
			return
		} else if joinFn, ok := join.(func() error); !ok {
			err = fmt.Errorf(
				"%w: %s not found", errLibFuncNotFound, JOIN_FUNC_NAME,
			)
			return
		} else {
			lib.joinFn = joinFn
		}

		runtime.SetFinalizer(lib, func(plugin *PluginLib) {
			plugin.cancel()

			plugin.unloadOnce.Do(func() {
				plugin.plugin = nil
				plugin.initFn = nil
				plugin.reportFn = nil
				plugin.joinFn = nil
			})
		})
	})

	return
}
