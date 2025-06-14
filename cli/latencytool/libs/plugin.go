package libs

import (
	"context"
	"runtime"
	"sync"
)

type PluginLib struct {
	libPath    string
	loadOnce   sync.Once
	unloadOnce sync.Once
	initOnce   sync.Once

	ctx      context.Context
	cancel   context.CancelFunc
	cfgPath  string
	initFn   func(context.Context, string) error
	reportFn func(...string) error
}

func (lib *PluginLib) Init(cfgPath string) (err error) {
	lib.initOnce.Do(func() {
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

func (lib *PluginLib) Join() {}

func NewPlugin(ctx context.Context, libPath string) (lib *PluginLib, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	lib = &PluginLib{
		libPath: libPath,
	}

	lib.ctx, lib.cancel = context.WithCancel(ctx)

	lib.loadOnce.Do(func() {
		// TODO: load plugin
	})

	runtime.SetFinalizer(lib, func(plugin *PluginLib) {
		plugin.unloadOnce.Do(func() {
			// TODO: unload plugin
		})
	})

	return
}
