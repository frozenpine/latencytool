package libs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"plugin"
	"runtime"
	"sync"
)

const (
	INIT_FUNC_NAME   = "CreateInstance"
	REPORT_FUNC_NAME = "ReportFronts"
	JOIN_FUNC_NAME   = "Join"
)

type GoPluginLib struct {
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

func (lib *GoPluginLib) Init(ctx context.Context, cfgPath string) (err error) {
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

func (lib *GoPluginLib) ReportFronts(addrList ...string) error {
	return lib.reportFn(addrList...)
}

func (lib *GoPluginLib) Stop() {
	lib.cancel()
}

func (lib *GoPluginLib) Join() error {
	return lib.joinFn()
}

func NewGoPlugin(dirName, libName string) (lib *GoPluginLib, err error) {
	libDir := path.Join(dirName, libName)

	switch runtime.GOOS {
	case "linux":
		if err := os.Setenv("LD_LIBRARY_PATH", libDir); err != nil {
			return nil, err
		}

		slog.Info(
			"lib environment setted for linux",
			slog.String("LD_LIBRARY_PATH", os.Getenv("LD_LIBRARY_PATH")),
		)
	case "windows":
	default:
		return nil, errors.New("unsupported platform")
	}

	libPath := filepath.Join(libDir, libName+".plugin")

	lib = &GoPluginLib{
		libPath: libPath,
	}

	lib.loadOnce.Do(func() {
		if !registeredPlugins.CompareAndSwap(
			libName, nil, &pluginContainer{
				pluginType: GoPlugin,
				libDir:     libDir,
				name:       libName,
				plugin:     lib,
			},
		) {
			err = fmt.Errorf(
				"%w: plugin already loaded", errLibOpenFailed,
			)

			return
		}

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

		runtime.SetFinalizer(lib, func(plugin *GoPluginLib) {
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
