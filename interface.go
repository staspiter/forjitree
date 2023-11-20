package forjitree

import (
	"errors"
	"fmt"
	"strings"
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

	Log(msgType string, msg string)
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
			c.SetLastError(actionErr.Error())
			if c.GetBreakOnError() {
				c.Log("error", a.GetNode().Path()+": "+actionErr.Error())
				return actionErr
			}
		}
	}
	lastError := c.GetLastError()
	if lastError != "" {
		return errors.New(lastError)
	}
	return nil
}

func EvaluateContextValue(c Context, s string) any {
	return evaluateStringInternal(c, s, false)
}

func evaluateStringInternal(c Context, s string, evaluateThisValue bool) any {
	if strings.HasPrefix(s, ":") {
		return c.Get(strings.TrimPrefix(s, ":"))
	}

	containsUnescapedCurlyBrace := false
	for i := 0; i < len(s); i++ {
		if s[i] == '{' && (i == 0 || (s[i-1] != '\\' && s[i-1] != '$')) {
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
			if s[i] == '{' && (i == 0 || (s[i-1] != '\\' && s[i-1] != '$')) {
				lastOpenBracket = i
			} else if s[i] == '}' && (i == 0 || (s[i-1] != '\\' && s[i-1] != '$')) {
				if lastOpenBracket > -1 {
					stringInsideBrackets := s[lastOpenBracket+1 : i]
					if !strings.ContainsAny(stringInsideBrackets, "\n\r\t") {
						s1 = s1[:len(s1)-(i-lastOpenBracket)] + fmt.Sprintf("%v", evaluateStringInternal(c, s[lastOpenBracket+1:i], true))
						substituted = true
						lastOpenBracket = -1
						continue
					}
					lastOpenBracket = -1
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
