package forjitree

import (
	"errors"
	"fmt"
	"strings"

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

	Get(key string) any
	Set(key string, value any)
	ExposeFields() map[string]any
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

func RunActions(actions []Action, c Context) error {
	for _, a := range actions {
		actionErr := a.Call(c)
		if actionErr != nil {
			log.Error().Str("group", "action").Str("node", a.GetNode().Path()).Err(actionErr).Msg("Action error")
			c.SetLastError(actionErr.Error())
			if c.GetBreakOnError() {
				return actionErr
			}
		}
	}
	return errors.New(c.GetLastError())
}

func EvaluateContextValue(c Context, s string) any {
	return evalutateStringInternal(c, s, false)
}

func evalutateStringInternal(c Context, s string, evaluateThisValue bool) any {
	if strings.HasPrefix(s, ":") {
		return c.Get(strings.TrimPrefix(s, ":"))
	}

	containsUnescapedCurlyBrace := false
	for i := 0; i < len(s); i++ {
		if s[i] == '{' && (i == 0 || s[i-1] != '\\') {
			containsUnescapedCurlyBrace = true
		}
	}
	if !containsUnescapedCurlyBrace && evaluateThisValue {
		return c.Get(s)
	}

	// Substitute all {XYZ} blocks with values
	for {
		s1 := ""
		substituted := false
		lastOpenBracket := -1
		for i := 0; i < len(s); i++ {
			if s[i] == '{' && (i == 0 || s[i-1] != '\\') {
				lastOpenBracket = i
			} else if s[i] == '}' && (i == 0 || s[i-1] != '\\') {
				if lastOpenBracket > -1 {
					s1 = s1[:len(s1)-(i-lastOpenBracket)] + fmt.Sprintf("%v", evalutateStringInternal(c, s[lastOpenBracket+1:i], true))
					lastOpenBracket = -1
					substituted = true
					continue
				}
			}
			s1 += string(s[i])
		}
		s = s1
		if !substituted {
			break
		}
	}

	return s
}
