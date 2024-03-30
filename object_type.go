package forjitree

import (
	"fmt"
	"plugin"
	"reflect"
	"regexp"
	"strings"
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

var pluginRegex = regexp.MustCompile(`syms\:map\[(.*)\]`)

func NewObjectTypesFromPlugin(pluginFilename string) ([]*ObjectType, error) {
	p, err := plugin.Open(pluginFilename)
	if err != nil {
		return nil, fmt.Errorf("loading plugin error %s: %s", pluginFilename, err)
	}

	pluginStr := fmt.Sprintf("%+v\n", p)
	if !pluginRegex.MatchString(pluginStr) {
		return nil, fmt.Errorf("loading plugin error %s: plugin does not contain syms:map[]", pluginFilename)
	}

	exportedFunctionsSubmatch := pluginRegex.FindStringSubmatch(pluginStr)
	exportedFunctionsStr := exportedFunctionsSubmatch[1]
	exportedFunctions := strings.Split(exportedFunctionsStr, " ")

	objectTypes := []*ObjectType{}
	for _, exportedFunctionStr := range exportedFunctions {
		exportedFunctionName := strings.Split(exportedFunctionStr, ":")[0]

		if !strings.HasPrefix(exportedFunctionName, "New") {
			continue
		}

		s, err := p.Lookup(exportedFunctionName)
		if err != nil {
			return nil, fmt.Errorf("loading plugin error %s: Could not lookup %s function", pluginFilename, exportedFunctionName)
		}

		newFunc, ok := s.(NewObjectFunc)
		if !ok {
			continue
			//return nil, fmt.Errorf("loading plugin error %s: %s function should match 'func() Object'", pluginFilename, exportedFunctionName)
		}

		objectTypes = append(objectTypes, &ObjectType{
			Name:          strings.TrimPrefix(exportedFunctionName, "New"),
			newObjectFunc: newFunc,
		})
	}

	return objectTypes, nil
}

func (t *ObjectType) createObject(node Node) Object {
	return t.newObjectFunc(node)
}

func (t *ObjectType) setField(n *node, fieldName string, fieldValue any) {
	f := n.objReflect.Elem().FieldByName(Capitalize(fieldName))
	if f.IsValid() && f.CanSet() {

		fieldValueType := reflect.TypeOf(fieldValue)
		fieldType := f.Type()

		if fieldValue == nil {
			if fieldType.Kind() == reflect.Interface || fieldType.Kind() == reflect.Pointer || fieldType.Kind() == reflect.Map || fieldType.Kind() == reflect.Array {
				f.Set(NilReflectValue)
			} else {
				f.Set(reflect.Zero(fieldType))
			}
		} else if fieldValueType != nil && fieldValueType.ConvertibleTo(fieldType) {
			f.Set(reflect.ValueOf(fieldValue).Convert(fieldType))
		}
	}

	n.obj.Updated(fieldName, fieldValue)
}
