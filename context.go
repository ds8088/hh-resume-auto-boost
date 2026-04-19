package main

import "context"

// AppContext is an enriched implementation of context.Context.
type AppContext struct {
	context.Context
	Cfg Config
}
