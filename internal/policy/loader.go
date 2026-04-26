package policy

import (
	"errors"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/ojuschugh1/aura/pkg/types"
)

// Load reads a PolicyConfig from a TOML file; returns DefaultConfig if the file doesn't exist.
func Load(path string) (*types.PolicyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := DefaultConfig()
			return &cfg, nil
		}
		return nil, err
	}
	var cfg types.PolicyConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Watch monitors path for changes and calls onChange within 5 seconds of a modification.
// Returns a stop function and any setup error.
func Watch(path string, onChange func(*types.PolicyConfig)) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return nil, err
	}

	stop := make(chan struct{})
	go func() {
		defer watcher.Close()
		for {
			select {
			case <-stop:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					// debounce: wait up to 5s then reload
					timer := time.NewTimer(100 * time.Millisecond)
					select {
					case <-timer.C:
					case <-stop:
						timer.Stop()
						return
					}
					cfg, err := Load(path)
					if err == nil {
						onChange(cfg)
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return func() { close(stop) }, nil
}
