package libs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"sync"
)

const (
	SET_LOGGER_FUNC_NAME = "SetLogger"
	INIT_FUNC_NAME       = "Init"
	REPORT_FUNC_NAME     = "ReportFronts"
	SEATS_FUNC_NAME      = "Seats"
	PRIORITY_FUNC_NAME   = "Priority"
	DESTORY_FUNC_NAME    = "Release"
	JOIN_FUNC_NAME       = "Join"
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

	loggerFn   func(slog.Level, string, int, int) error
	initFn     func(context.Context, string) error
	reportFn   func(...string) error
	seatsFn    func() []Seat
	priorityFn func() [][]int
	joinFn     func() error
	releaseFn  func()
}

func (goLib *GoPluginLib) SetLogger(
	lvl slog.Level, logFile string, logSize, logKeep int,
) error {
	return goLib.loggerFn(lvl, logFile, logSize, logKeep)
}

func (goLib *GoPluginLib) Init(ctx context.Context, cfgPath string) (err error) {
	goLib.initOnce.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		goLib.ctx, goLib.cancel = context.WithCancel(ctx)

		if err = goLib.initFn(goLib.ctx, cfgPath); err != nil {
			return
		}

		goLib.cfgPath = cfgPath

		slog.Info(
			"go plugin initialized",
			slog.String("plugin", goLib.libPath),
			slog.String("config", goLib.cfgPath),
		)
	})

	return
}

func (goLib *GoPluginLib) ReportFronts(addrList ...string) error {
	return goLib.reportFn(addrList...)
}

func (goLib *GoPluginLib) Seats() []Seat {
	return goLib.seatsFn()
}

func (goLib *GoPluginLib) Priority() [][]int {
	return goLib.priorityFn()
}

func (goLib *GoPluginLib) Stop() {
	goLib.releaseFn()
	goLib.cancel()
}

func (goLib *GoPluginLib) Join() error {
	return goLib.joinFn()
}

func NewGoPlugin(dirName, libName string) (container *PluginContainer, err error) {
	libIdentity := strings.SplitN(libName, ".", 2)

	switch runtime.GOOS {
	case "linux":
	default:
		return nil, errors.New("unsupported platform")
	}

	libPath := filepath.Join(dirName, libIdentity[0]+".plugin")

	lib := &GoPluginLib{
		libPath: libPath,
	}

	lib.loadOnce.Do(func() {
		if _, loaded := registeredPlugins.LoadOrStore(
			libName, &PluginContainer{
				pluginType: GoPlugin,
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

		if lib.plugin, err = plugin.Open(lib.libPath); err != nil {
			err = errors.Join(errLibOpenFailed, err)
			return
		}

		if logger, failed := lib.plugin.Lookup(
			SET_LOGGER_FUNC_NAME,
		); failed != nil {
			err = errors.Join(errLibFuncNotFound, failed)
			return
		} else if loggerFn, ok := logger.(func(
			slog.Level, string, int, int,
		) error); !ok {
			err = fmt.Errorf(
				"%w: %s not found", errLibFuncNotFound, SET_LOGGER_FUNC_NAME,
			)
			return
		} else {
			lib.loggerFn = loggerFn
		}

		if init, failed := lib.plugin.Lookup(INIT_FUNC_NAME); failed != nil {
			err = errors.Join(errLibFuncNotFound, failed)
			return
		} else if initFn, ok := init.(func(
			context.Context, string,
		) error); !ok {
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

		if seat, failed := lib.plugin.Lookup(SEATS_FUNC_NAME); failed != nil {
			err = errors.Join(errLibFuncNotFound, failed)
			return
		} else if seatFn, ok := seat.(func() []Seat); !ok {
			err = fmt.Errorf(
				"%w: %s not found", errLibFuncNotFound, SEATS_FUNC_NAME,
			)
			return
		} else {
			lib.seatsFn = seatFn
		}

		if pri, failed := lib.plugin.Lookup(PRIORITY_FUNC_NAME); failed != nil {
			err = errors.Join(errLibFuncNotFound, failed)
			return
		} else if priFn, ok := pri.(func() [][]int); !ok {
			err = fmt.Errorf(
				"%w: %s not found", errLibFuncNotFound, SEATS_FUNC_NAME,
			)
			return
		} else {
			lib.priorityFn = priFn
		}

		if stop, failed := lib.plugin.Lookup(DESTORY_FUNC_NAME); failed != nil {
			err = errors.Join(errLibFuncNotFound, failed)
			return
		} else if stopFn, ok := stop.(func()); !ok {
			err = fmt.Errorf(
				"%w: %s not found", errLibFuncNotFound, DESTORY_FUNC_NAME,
			)
			return
		} else {
			lib.releaseFn = stopFn
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

		container, err = GetRegisteredPlugin(libName)
	})

	return
}
