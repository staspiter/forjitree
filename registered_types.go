package forjitree

import "strings"

type registeredTypesSingleton struct {
	types       map[string]*ObjectType
	pluginTypes map[string][]*ObjectType
}

var RegisteredTypes = &registeredTypesSingleton{
	types:       map[string]*ObjectType{},
	pluginTypes: map[string][]*ObjectType{},
}

func (r *registeredTypesSingleton) RegisterType(newObjectFunc NewObjectFunc, name string) *ObjectType {
	t := NewObjectType(newObjectFunc, name)
	r.types[name] = t
	return t
}

func (r *registeredTypesSingleton) GetTypes(types []string) ([]*ObjectType, error) {
	result := []*ObjectType{}
	for _, t := range types {
		t = strings.TrimSpace(t)
		if ot, ok := r.types[t]; ok {
			result = append(result, ot)
		} else if types, ok := r.pluginTypes[t]; ok {
			result = append(result, types...)
		} else {
			types, err := NewObjectTypesFromPlugin(t)
			if err != nil {
				return nil, err
			}
			r.pluginTypes[t] = types
			for _, ot := range types {
				r.types[ot.Name] = ot
			}
			result = append(result, types...)
		}
	}
	return result, nil
}

func (r *registeredTypesSingleton) GetTypesFromStr(types string) ([]*ObjectType, error) {
	return r.GetTypes(strings.Split(types, ","))
}
