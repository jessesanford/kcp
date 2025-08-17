package plugins

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"github.com/kcp-dev/kcp/pkg/interfaces/syncer"
)

// Plugin represents a syncer plugin
type Plugin interface {
	// Name returns the plugin name
	Name() string

	// Version returns the plugin version
	Version() string

	// Initialize sets up the plugin
	Initialize(ctx context.Context, config *PluginConfig) error

	// Start begins plugin operation
	Start(ctx context.Context) error

	// Stop halts the plugin
	Stop() error

	// Healthy checks plugin health
	Healthy() bool
}

// PluginConfig contains plugin configuration
type PluginConfig struct {
	// Name of the plugin
	Name string

	// Settings for the plugin
	Settings map[string]interface{}

	// UpstreamClient for KCP
	UpstreamClient dynamic.ClusterInterface

	// DownstreamClient for physical cluster
	DownstreamClient dynamic.Interface

	// Scheme for object decoding
	Scheme *runtime.Scheme
}

// PluginRegistry manages plugins
type PluginRegistry interface {
	// Register adds a plugin
	Register(plugin Plugin) error

	// Unregister removes a plugin
	Unregister(name string) error

	// Get retrieves a plugin
	Get(name string) (Plugin, error)

	// List returns all plugins
	List() []Plugin

	// LoadPlugin dynamically loads a plugin
	LoadPlugin(path string) (Plugin, error)
}

// HookPoint represents where hooks can be attached
type HookPoint string

const (
	HookPointPreSync    HookPoint = "PreSync"
	HookPointPostSync   HookPoint = "PostSync"
	HookPointPreDelete  HookPoint = "PreDelete"
	HookPointPostDelete HookPoint = "PostDelete"
	HookPointError      HookPoint = "Error"
)

// Hook is called at specific points
type Hook interface {
	// Name of the hook
	Name() string

	// Execute runs the hook
	Execute(ctx context.Context, point HookPoint, data interface{}) error

	// ShouldExecute determines if hook should run
	ShouldExecute(point HookPoint, data interface{}) bool
}

// HookManager manages hooks
type HookManager interface {
	// RegisterHook adds a hook
	RegisterHook(point HookPoint, hook Hook) error

	// UnregisterHook removes a hook
	UnregisterHook(point HookPoint, name string) error

	// ExecuteHooks runs all hooks for a point
	ExecuteHooks(ctx context.Context, point HookPoint, data interface{}) error
}

// PluginLoader loads plugins dynamically
type PluginLoader interface {
	// Load loads a plugin from path
	Load(path string) (Plugin, error)

	// Unload removes a loaded plugin
	Unload(name string) error

	// Reload reloads a plugin
	Reload(name string) error
}