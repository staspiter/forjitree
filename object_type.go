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

	redirect *bool
}

func NewObjectType(newObjectFunc NewObjectFunc, name string) *ObjectType {
	return &ObjectType{
		Name:          name,
		newObjectFunc: newObjectFunc,
		redirect:      nil,
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

		if fieldValue == nil && (fieldType.Kind() == reflect.Interface || fieldType.Kind() == reflect.Pointer || fieldType.Kind() == reflect.Map || fieldType.Kind() == reflect.Array) {
			f.Set(NilReflectValue)

		} else if fieldType.Kind() == reflect.Int && fieldValueType != nil && fieldValueType.Kind() == reflect.Float32 {
			// Allow float32 to int assignments with truncation
			f.Set(reflect.ValueOf(int(fieldValue.(float32))))

		} else if fieldType.Kind() == reflect.Int && fieldValueType != nil && fieldValueType.Kind() == reflect.Float64 {
			// Allow float64 to int assignments with truncation
			f.Set(reflect.ValueOf(int(fieldValue.(float64))))

		} else if fieldType.Kind() == reflect.Int64 && fieldValueType != nil && fieldValueType.Kind() == reflect.Float32 {
			// Allow float32 to int64 assignments with truncation
			f.Set(reflect.ValueOf(int64(fieldValue.(float32))))

		} else if fieldType.Kind() == reflect.Int64 && fieldValueType != nil && fieldValueType.Kind() == reflect.Float64 {
			// Allow float64 to int64 assignments with truncation
			f.Set(reflect.ValueOf(int64(fieldValue.(float64))))

		} else if fieldValue != nil && fieldValueType != nil && fieldValueType.AssignableTo(fieldType) {
			f.Set(reflect.ValueOf(fieldValue))
		}
	}

	n.obj.Updated(fieldName, fieldValue)
}

func (t *ObjectType) callRedirect(n *node) []*node {
	var m reflect.Value

	// Cache Redirect function presence
	if t.redirect == nil || *t.redirect {
		if !n.objReflect.IsValid() || n.objReflect.IsZero() {
			return []*node{n}
		}
		m = n.objReflect.MethodByName("Redirect")
		var b bool = m.IsValid() && !m.IsZero()
		t.redirect = &b
		if !*t.redirect {
			return []*node{n}
		}
	} else {
		return []*node{n}
	}

	callResult := m.Call(nil)
	if len(callResult) == 1 {
		nodes := callResult[0].Interface().([]Node)
		result := make([]*node, len(nodes))
		for i, n2 := range nodes {
			result[i] = n2.internalNode()
		}
		return result
	} else {
		return []*node{n}
	}
}
