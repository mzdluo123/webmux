package hub

import (
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/session"
	"github.com/rainchan/webmux/internal/wire"
)

// Options contains prebound, typed controller contributions. Nil contributions
// leave the terminal host fully usable without constructing optional services.
type Options struct {
	// Project reads only precomputed caches: list capture invokes it at the
	// manager's ordered publication boundary, without a session lock.
	Project           func(session.Info) contract.SessionInfo
	Terminal          func(id string) (allowed bool, destination string)
	SubscribeForwards func() (<-chan struct{}, func())
	Forwards          func() []contract.Forward
}

func (o Options) project(info session.Info) contract.SessionInfo {
	if o.Project != nil {
		return o.Project(info)
	}
	return wire.Session(info)
}

func options(in []Options) Options {
	if len(in) > 0 {
		return in[0]
	}
	return Options{}
}
