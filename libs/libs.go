package libs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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

type PluginContainer struct {
	pluginType

	plugin Plugin
	libDir string
	name   string
}

func (c *PluginContainer) Plugin() Plugin {
	return c.plugin
}

func (c *PluginContainer) Name() string {
	return c.name
}

func (c *PluginContainer) String() string {
	buff := bytebufferpool.Get()
	defer bytebufferpool.Put(buff)

	buff.WriteString("Plugin{Name:")
	buff.WriteString(c.name)
	buff.WriteString(" LibDir:")
	buff.WriteString(c.libDir)
	buff.WriteString(" Type:")
	buff.WriteString(string(c.pluginType))
	buff.WriteString("}")

	return buff.String()
}

func (c *PluginContainer) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"PluginType": c.pluginType,
		"Name":       c.name,
		"LibDir":     c.libDir,
	})
}

var (
	registeredPlugins sync.Map
)

func NewPlugin(libDir, name string) (container *PluginContainer, err error) {
	var libType = CPlugin

	if strings.ToLower(runtime.GOOS) != "linux" {
		// go plugin only supported in linux
		// double check environment for correct plugin type
		libType = CPlugin
	} else {
		switch {
		case strings.HasPrefix(name, "Go."):
			libType = GoPlugin
			name = strings.TrimPrefix(name, "Go.")
		case strings.HasPrefix(name, "C."):
			libType = CPlugin
			name = strings.TrimPrefix(name, "C.")
		}
	}

	switch libType {
	case GoPlugin:
		container, err = NewGoPlugin(libDir, name)
		return
	case CPlugin:
		container, err = NewCPlugin(libDir, name)
		return
	default:
		return nil, ErrInvalidPluginType
	}
}

func GetRegisteredPlugin(name string) (*PluginContainer, error) {
	v, exist := registeredPlugins.Load(name)
	if !exist {
		return nil, errLibNotRegistered
	}
	plugin, ok := v.(*PluginContainer)

	if !ok {
		registeredPlugins.Delete(name)
		return nil, errInvalidRegLib
	}

	return plugin, nil
}

func GetAndUnRegisterPlugin(name string) (plugin *PluginContainer, err error) {
	defer func() {
		if err != nil {
			return
		}

		if plugin.pluginType != GoPlugin {
			registeredPlugins.Delete(name)
		} else {
			slog.Warn(
				"go plugin can not be unloaded",
				slog.String("name", name),
			)
		}
	}()

	plugin, err = GetRegisteredPlugin(name)

	return
}

func RangePlugins(fn func(name string, container *PluginContainer) error) (err error) {
	registeredPlugins.Range(func(key, value any) bool {
		name, _ := key.(string)
		container, _ := value.(*PluginContainer)

		if err = fn(name, container); err != nil {
			return false
		}

		return true
	})

	return
}
