// Package plugin provides the plugin system for extending Radiant Harness.
// Plugins can register custom protocols, validators, and hooks.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
)

// Plugin represents a loaded plugin.
type Plugin struct {
	Name        string
	Version     string
	Description string
	Path        string
	Instance    interface{}
}

// PluginManifest is the plugin's metadata file.
type PluginManifest struct {
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description" yaml:"description"`
	Entry       string `json:"entry" yaml:"entry"` // .so/.dylib file
	Author      string `json:"author" yaml:"author"`
	MinVersion  string `json:"min_version" yaml:"min_version"`
}

// PluginManager manages plugin discovery and loading.
type PluginManager struct {
	mu      sync.Mutex
	plugins map[string]*Plugin
	paths   []string
	hooks   *HookRegistry
}

// NewPluginManager creates a new plugin manager.
func NewPluginManager(searchPaths ...string) *PluginManager {
	if len(searchPaths) == 0 {
		home, _ := os.UserHomeDir()
		searchPaths = []string{
			filepath.Join(home, ".radiant-harness", "plugins"),
			".radiant-harness/plugins",
		}
	}

	return &PluginManager{
		plugins: make(map[string]*Plugin),
		paths:   searchPaths,
		hooks:   NewHookRegistry(),
	}
}

// Discover finds all plugins in the search paths.
func (pm *PluginManager) Discover() []string {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var discovered []string
	for _, path := range pm.paths {
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				manifestPath := filepath.Join(path, entry.Name(), "plugin.yaml")
				if _, err := os.Stat(manifestPath); err == nil {
					discovered = append(discovered, filepath.Join(path, entry.Name()))
				}
			}
		}
	}
	return discovered
}

// Load loads a plugin from a directory.
func (pm *PluginManager) Load(dir string) (*Plugin, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	manifestPath := filepath.Join(dir, "plugin.yaml")
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	// Check if already loaded
	if existing, ok := pm.plugins[manifest.Name]; ok {
		return existing, nil
	}

	p := &Plugin{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
		Path:        dir,
	}

	// Try to load native plugin (.so/.dylib)
	entryPath := filepath.Join(dir, manifest.Entry)
	if _, err := os.Stat(entryPath); err == nil {
		pl, err := plugin.Open(entryPath)
		if err != nil {
			return nil, fmt.Errorf("open plugin %s: %w", manifest.Entry, err)
		}

		// Look for Register function
		registerSym, err := pl.Lookup("Register")
		if err == nil {
			if register, ok := registerSym.(func(*PluginContext) error); ok {
				ctx := &PluginContext{
					PluginName: manifest.Name,
					Hooks:      pm.hooks,
				}
				if err := register(ctx); err != nil {
					return nil, fmt.Errorf("register plugin %s: %w", manifest.Name, err)
				}
			}
		}

		p.Instance = pl
	}

	pm.plugins[manifest.Name] = p
	return p, nil
}

// LoadAll discovers and loads all plugins.
func (pm *PluginManager) LoadAll() ([]*Plugin, []error) {
	dirs := pm.Discover()
	var plugins []*Plugin
	var errors []error

	for _, dir := range dirs {
		p, err := pm.Load(dir)
		if err != nil {
			errors = append(errors, err)
		} else {
			plugins = append(plugins, p)
		}
	}

	return plugins, errors
}

// Get returns a loaded plugin by name.
func (pm *PluginManager) Get(name string) *Plugin {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.plugins[name]
}

// List returns all loaded plugins.
func (pm *PluginManager) List() []*Plugin {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	var plugins []*Plugin
	for _, p := range pm.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// Hooks returns the hook registry.
func (pm *PluginManager) Hooks() *HookRegistry {
	return pm.hooks
}

// ── Plugin Context ──

// PluginContext is passed to plugins during registration.
type PluginContext struct {
	PluginName string
	Hooks      *HookRegistry
}

// RegisterHook registers a hook with the plugin manager.
func (ctx *PluginContext) RegisterHook(name string, handler HookHandler) {
	ctx.Hooks.Add(name, handler)
}

// ── Hook Registry ──

// HookHandler is a function called during a hook.
type HookHandler func(ctx *HookContext) error

// HookContext provides context to hook handlers.
type HookContext struct {
	HookName string
	Data     map[string]interface{}
}

// HookRegistry manages hooks.
type HookRegistry struct {
	mu       sync.Mutex
	handlers map[string][]HookHandler
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		handlers: make(map[string][]HookHandler),
	}
}

// Add adds a handler for a hook.
func (hr *HookRegistry) Add(name string, handler HookHandler) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.handlers[name] = append(hr.handlers[name], handler)
}

// Execute runs all handlers for a hook.
func (hr *HookRegistry) Execute(name string, data map[string]interface{}) error {
	hr.mu.Lock()
	handlers := hr.handlers[name]
	hr.mu.Unlock()

	ctx := &HookContext{HookName: name, Data: data}
	for _, handler := range handlers {
		if err := handler(ctx); err != nil {
			return fmt.Errorf("hook %s: %w", name, err)
		}
	}
	return nil
}

// Has returns true if any handlers are registered for a hook.
func (hr *HookRegistry) Has(name string) bool {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	return len(hr.handlers[name]) > 0
}

// List returns all registered hook names.
func (hr *HookRegistry) List() []string {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	var names []string
	for name := range hr.handlers {
		names = append(names, name)
	}
	return names
}

// ── Standard Hooks ──

const (
	HookPreScaffold   = "pre_scaffold"
	HookPostScaffold  = "post_scaffold"
	HookPreImplement  = "pre_implement"
	HookPostImplement = "post_implement"
	HookPreValidate   = "pre_validate"
	HookPostValidate  = "post_validate"
	HookPreRun        = "pre_run"
	HookPostRun       = "post_run"
	HookOnTaskStart   = "on_task_start"
	HookOnTaskEnd     = "on_task_end"
	HookOnError       = "on_error"
)

// ── Plugin Loader for Go plugins ──

// ProtocolPlugin is the interface plugins implement to add agent protocols.
type ProtocolPlugin interface {
	Name() string
	Command() string
	BuildArgs(prompt string) []string
	ValidateConfig() error
}

// ValidatorPlugin is the interface plugins implement to add validators.
type ValidatorPlugin interface {
	Name() string
	Validate(specDir string) (bool, []string, error)
}

// ── YAML manifest reader ──

func readManifest(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	manifest := &PluginManifest{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" || line == "---" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			manifest.Name = value
		case "version":
			manifest.Version = value
		case "description":
			manifest.Description = value
		case "entry":
			manifest.Entry = value
		case "author":
			manifest.Author = value
		case "min_version":
			manifest.MinVersion = value
		}
	}

	if manifest.Name == "" {
		return nil, fmt.Errorf("plugin manifest missing 'name'")
	}

	return manifest, nil
}
