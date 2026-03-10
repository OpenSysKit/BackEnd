//go:build !windows

package main

import (
	"fmt"
	"time"
)

type frontendGuard struct {
	done chan struct{}
}

func newFrontendGuard() (*frontendGuard, error) {
	return nil, fmt.Errorf("前端守护仅支持 Windows")
}

func (g *frontendGuard) Start() error             { return fmt.Errorf("not supported") }
func (g *frontendGuard) Done() <-chan struct{}    { return g.done }
func (g *frontendGuard) Kill()                    {}
func (g *frontendGuard) pidOf() int               { return -1 }
func waitForPipe(_ string, _ time.Duration) error { return fmt.Errorf("not supported") }
