package forjitree

import (
	"path/filepath"
	"strings"
	"sync"
)

type registeredTypesSingleton struct {
	types       map[string]*ObjectType
	pluginTypes map[string][]*ObjectType
	mu          sync.Mutex
}

var RegisteredTypes = &registeredTypesSingleton{
	types:       map[string]*ObjectType{},
	pluginTypes: map[string][]*ObjectType{},
}

func (r *registeredTypesSingleton) RegisterType(newObjectFunc NewObjectFunc, name string) *ObjectType {
	r.mu.Lock()
	defer r.mu.Unlock()

	t := NewObjectType(newObjectFunc, name)
	r.types[name] = t
	return t
}

func (r *registeredTypesSingleton) GetTypes(types []string) ([]*ObjectType, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := []*ObjectType{}
	for _, t := range types {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		pluginFilename := filepath.Base(t)

		if ot, ok := r.types[t]; ok {
			result = append(result, ot)
		} else if types, ok := r.pluginTypes[pluginFilename]; ok {
			result = append(result, types...)
		} else {
			types, err := NewObjectTypesFromPlugin(t)
			if err != nil {
				return nil, err
			}
			r.pluginTypes[pluginFilename] = types
			result = append(result, types...)
		}
	}
	return result, nil
}

func (r *registeredTypesSingleton) GetTypesFromStr(types string) ([]*ObjectType, error) {
	return r.GetTypes(strings.Split(types, ","))
}

func (r *registeredTypesSingleton) GetAllNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := []string{}
	for t := range r.types {
		result = append(result, t)
	}
	return result
}

func (r *registeredTypesSingleton) IsTypeRegistered(typename string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.types[typename]
	if !exists {
		for _, types := range r.pluginTypes {
			for _, t := range types {
				if t.Name == typename {
					exists = true
					break
				}
			}
		}
	}
	return exists
}
