package libs

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"

	"github.com/valyala/bytebufferpool"
)

type pluginType string

const (
	GoPlugin pluginType = "golib"
	CPlugin  pluginType = "clib"
)

var (
	errLibOpenFailed    = errors.New("open lib failed")
	errLibFuncNotFound  = errors.New("lib func not found")
	errLibNotRegistered = errors.New("lib not registered")
	errInvalidRegLib    = errors.New("invalid registered lib")
)

type Plugin interface {
	Init(context.Context, string) error
	Stop()
	Join() error
	ReportFronts(...string) error
}

type pluginContainer struct {
	pluginType

	plugin Plugin
	libDir string
	name   string
}

func (c *pluginContainer) Plugin() Plugin {
	return c.plugin
}

func (c *pluginContainer) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	return buff.String()
}

var (
	registeredPlugins sync.Map
)

func NewPlugin(libDir, name string) (Plugin, error) {
	var (
		libType = GoPlugin
		plugin  Plugin
		err     error
	)

	if strings.HasPrefix(name, "C.") {
		libType = CPlugin
		name = strings.TrimPrefix(name, "C.")
	} else if strings.ToLower(runtime.GOOS) != "linux" {
		// go plugin only supported in linux
		// double check environment for correct plugin type
		libType = CPlugin
	}

	switch libType {
	case GoPlugin:
		plugin, err = NewGoPlugin(libDir, name)
	case CPlugin:
		plugin, err = NewCPlugin(libDir, name)
	default:
		return nil, ErrInvalidPluginType
	}

	if err != nil {
		return nil, err
	} else {
		return plugin, nil
	}
}

func GetRegisteredPlugin(name string) (*pluginContainer, error) {
	v, exist := registeredPlugins.Load(name)
	if !exist {
		return nil, errLibNotRegistered
	}
	plugin, ok := v.(*pluginContainer)

	if !ok {
		registeredPlugins.Delete(name)
		return nil, errInvalidRegLib
	}

	return plugin, nil
}

func GetAndUnRegisterPlugin(name string) (plugin *pluginContainer, err error) {
	defer func() {
		if err != nil {
			return
		}

		if plugin.pluginType != GoPlugin {
			registeredPlugins.Delete(name)
		} else {
			err = errors.New("")
		}
	}()

	plugin, err = GetRegisteredPlugin(name)

	return
}
