package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings from the TUI panel are saved in <user config dir>/ytmanager/config.json; the format
// is not, because it lives in each destination's data.music.
func optionsPath() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "ytmanager", "config.json"), err
}

func LoadOptions() Options {
	o := Options{Jobs: 3}
	if p, err := optionsPath(); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			_ = json.Unmarshal(b, &o)
		}
	}
	return o
}

func SaveOptions(o Options) error {
	p, err := optionsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(o, "", "  ")
	return os.WriteFile(p, b, 0o644)
}

// SetOptions changes download, tagging and folder settings for the next operation.
// Do not call it while a sync or rearrange runs.
func (e *Engine) SetOptions(o Options) {
	if o.Jobs < 1 {
		o.Jobs = 1
	}
	e.opt = o
}

func (e *Engine) Options() Options { return e.opt }

func (e *Engine) Jobs() int { return e.opt.Jobs }
