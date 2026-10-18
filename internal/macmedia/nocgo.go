//go:build darwin && !cgo

package macmedia

import "context"

func Start(context.Context, backend) (*Service, error) { return nil, ErrUnavailable }
func Run(run func() error) error                       { return run() }
