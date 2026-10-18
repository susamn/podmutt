//go:build !darwin

package macmedia

import "context"

func Start(context.Context, backend) (*Service, error) { return nil, nil }
func Run(run func() error) error                       { return run() }
