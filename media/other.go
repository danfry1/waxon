//go:build !darwin && !linux

package media

func start(func(Event)) (Controller, error) { return nil, ErrUnsupported }

func runMain(fn func() int) int { return fn() }
