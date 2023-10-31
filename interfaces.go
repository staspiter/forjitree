package forjitree

import (
	"errors"

	"github.com/rs/zerolog/log"
)

type Object interface {
	GetNode() Node
	Created()
	CreatedChildren()
	CreatedTree()
	Destroyed()
	Updated(string, any)
}

type NewObjectFunc = func(Node) Object

type PluginsGetTypesFunc = func() []string

type Datasource interface {
	Object

	Connect() error
	Disconnect() error

	Get(query any) (any, error)
	Set(query any) (map[string][]int, error)
	Delete(query any) (map[string][]int, error)
	Clear() error
	Watch(query any, watcherId string) (any, error)
}

type Context interface {
	GetSchema() Schema

	GetBreakOnError() bool
	SetBreakOnError(bool)

	GetLastError() string
	SetLastError(string)
}

type Schema interface {
	Object

	NewContext(event any) Context

	DefaultDatasource() Datasource
	GetTypes() []string
}

type Action interface {
	Object

	Call(Context) error
}

func RunActions(actions []Action, ctx Context) error {
	for _, a := range actions {
		actionErr := a.Call(ctx)
		if actionErr != nil {
			log.Error().Str("group", "action").Str("node", a.GetNode().Path()).Err(actionErr).Msg("Action error")
			ctx.SetLastError(actionErr.Error())
			if ctx.GetBreakOnError() {
				return actionErr
			}
		}
	}
	return errors.New(ctx.GetLastError())
}
