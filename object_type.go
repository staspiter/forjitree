package tree

import (
	"plugin"
	"reflect"

	"github.com/rs/zerolog/log"
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

func NewObjectTypesFromPlugin(pluginFilename string) []*ObjectType {
	p, err := plugin.Open(pluginFilename)
	if err != nil {
		log.Error().Err(err).Msgf("Loading plugin error %s: %s", pluginFilename, err)
		return nil
	}
	s, err := p.Lookup("GetTypes")
	if err != nil {
		log.Error().Err(err).Msgf("Loading plugin error %s: GetTypes function was not found: %s", pluginFilename, err)
		return nil
	}
	getTypesFunc, ok := s.(PluginsGetTypesFunc)
	if !ok {
		log.Error().Err(err).Msgf("Loading plugin error %s: GetTypes function is invalid: %s", pluginFilename, err)
		return nil
	}
	typeNames := getTypesFunc()

	objectTypes := []*ObjectType{}
	for _, typeName := range typeNames {
		s, err = p.Lookup("New" + typeName)
		if err != nil {
			log.Error().Err(err).Msgf("Loading plugin error %s: New%s function was not found", typeName, typeName)
			return nil
		}
		newFunc, ok := s.(NewObjectFunc)
		if !ok {
			log.Error().Err(err).Msgf("Loading plugin error %s: New%s function should match 'func() Object'", typeName, typeName)
			return nil
		}
		objectTypes = append(objectTypes, &ObjectType{
			Name:          typeName,
			newObjectFunc: newFunc,
		})
	}

	return objectTypes
}

func (t *ObjectType) createObject(node Node) Object {
	return t.newObjectFunc(node)
}

func (t *ObjectType) setField(n *node, fieldName string, fieldValue any) {
	f := n.objReflect.Elem().FieldByName(Capitalize(fieldName))
	if f.IsValid() && f.CanSet() {
		if fieldValue != nil && reflect.TypeOf(fieldValue).AssignableTo(f.Type()) {
			f.Set(reflect.ValueOf(fieldValue))
		}
	}

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
