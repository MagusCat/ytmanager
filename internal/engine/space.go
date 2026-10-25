package engine

import (
	"context"
	"fmt"
	"os"
	"time"

	"ytmanager/internal/device"
)

func (e *Engine) checkSpace(needed uint64) error {
	check := e.spaceCheck
	if check == nil {
		check = device.RequireSpace
	}
	for _, location := range []struct {
		name, path string
		needed     uint64
	}{
		{"destination", e.m.Root, needed}, {"local temporary storage", os.TempDir(), 0},
	} {
		if err := check(location.path, location.needed); err != nil {
			return fmt.Errorf("low space in %s; free space and sync to resume: %w", location.name, err)
		}
	}
	return nil
}

func (e *Engine) watchSpace(parent context.Context) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := e.checkSpace(0); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	return ctx, cancel
}
