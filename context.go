package main

import "context"

// AppContext is an enriched implementation of context.Context.
type AppContext struct {
	context.Context
	Cfg Config
}

func (ac *AppContext) WithCancel() (*AppContext, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ac)
	return &AppContext{
		Context: ctx,
		Cfg:     ac.Cfg,
	}, cancel
}
