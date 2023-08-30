package forjitree

import (
	"fmt"
	"plugin"
	"reflect"
)

type ObjectType struct {
	Name          string
	newObjectFunc NewObjectFunc
}

func NewObjectType(newObjectFunc NewObjectFunc, name string) *ObjectType {
	return &ObjectType{
		Name:          name,
		newObjectFunc: newObjectFunc,
	}
}

func NewObjectTypesFromPlugin(pluginFilename string) ([]*ObjectType, error) {
	p, err := plugin.Open(pluginFilename)
	if err != nil {
		return nil, fmt.Errorf("loading plugin error %s: %s", pluginFilename, err)
	}
	s, err := p.Lookup("GetTypes")
	if err != nil {
		return nil, fmt.Errorf("loading plugin error %s: GetTypes function was not found: %s", pluginFilename, err)
	}
	getTypesFunc, ok := s.(PluginsGetTypesFunc)
	if !ok {
		return nil, fmt.Errorf("loading plugin error %s: GetTypes function is invalid: %s", pluginFilename, err)
	}
	typeNames := getTypesFunc()

	objectTypes := []*ObjectType{}
	for _, typeName := range typeNames {
		s, err = p.Lookup("New" + typeName)
		if err != nil {
			return nil, fmt.Errorf("loading plugin error %s: New%s function was not found", typeName, typeName)
		}
		newFunc, ok := s.(NewObjectFunc)
		if !ok {
			return nil, fmt.Errorf("loading plugin error %s: New%s function should match 'func() Object'", typeName, typeName)
		}
		objectTypes = append(objectTypes, &ObjectType{
			Name:          typeName,
			newObjectFunc: newFunc,
		})
	}

	return objectTypes, nil
}

func (t *ObjectType) createObject(node Node) Object {
	return t.newObjectFunc(node)
}

func (t *ObjectType) setField(n *node, fieldName string, fieldValue any, callUpdated bool) {
	f := n.objReflect.Elem().FieldByName(Capitalize(fieldName))
	if f.IsValid() && f.CanSet() {
		if fieldValue != nil && reflect.TypeOf(fieldValue).AssignableTo(f.Type()) {
			f.Set(reflect.ValueOf(fieldValue))
		}
	}

	if callUpdated {
		m := n.objReflect.MethodByName("Updated")
		if m.IsValid() && !m.IsZero() {
			if fieldValue == nil {
				// A workaround for nil values, because reflect.Value does not support nil values
				m.Call([]reflect.Value{reflect.ValueOf(fieldName), NilReflectValue})
			} else {
				m.Call([]reflect.Value{reflect.ValueOf(fieldName), reflect.ValueOf(fieldValue)})
			}
		}
	}
}
