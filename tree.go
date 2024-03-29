package forjitree

import (
	"sync"
	"time"
)

type Tree struct {
	objectTypes map[string]*ObjectType
	rootNode    *node
	created     bool
	modified    bool
	name        string
	datasource  Datasource

	watchers               map[string]*watcher
	watchersMutex          sync.Mutex
	watchersCleanTimestamp time.Time
	watchersCleanInterval  float64
}

func New() *Tree {
	t := &Tree{
		objectTypes:            make(map[string]*ObjectType),
		created:                false,
		modified:               false,
		watchers:               make(map[string]*watcher),
		watchersCleanTimestamp: time.Now(),
		watchersCleanInterval:  60,
	}
	t.rootNode = newNode(t, nil, "")
	return t
}

func (t *Tree) Created() {
	if t.created {
		return
	}
	t.rootNode.callCreatedTree()
	t.created = true
}

func (t *Tree) Clear() {
	t.watchers = make(map[string]*watcher)
	t.rootNode.destroyObject(true)
	t.rootNode = newNode(t, nil, "")
	t.created = false
	t.modified = true
}

func (t *Tree) GetValue() any {
	return t.rootNode.getValue()
}

func (t *Tree) Set(data any) {
	modifiedNodes := t.rootNode.patch(data)

	// Call synchronize for modified nodes
	createdObjects := []*node{}
	for i := len(modifiedNodes) - 1; i >= 0; i-- {
		if modifiedNodes[i].synchronize() {
			createdObjects = append(createdObjects, modifiedNodes[i])
		}
	}

	// Call CreatedChildren
	for i := len(createdObjects) - 1; i >= 0; i-- {
		createdObjects[i].obj.CreatedChildren()
	}

	// Call CreatedTree if the tree has already been created
	if t.created {
		for i := 0; i < len(createdObjects); i++ {
			createdObjects[i].obj.CreatedTree()
		}
	}

	if len(modifiedNodes) > 0 {
		t.modified = true
	}

	// Merge with watchers changes
	t.watchersMutex.Lock()
	for _, w := range t.watchers {
		w.collectChanges(data)
	}
	t.watchersMutex.Unlock()
}

func (t *Tree) Watch(watcherId string) any {
	// Clean watchers routine - delete those ones which haven't been accessed for longer than watchersCleanInterval
	if time.Since(t.watchersCleanTimestamp).Seconds() > t.watchersCleanInterval/2 {
		t.watchersCleanTimestamp = time.Now()
		t.watchersMutex.Lock()
		for wid, w := range t.watchers {
			if time.Since(w.extractTimestamp).Seconds() > t.watchersCleanInterval {
				delete(t.watchers, wid)
			}
		}
		t.watchersMutex.Unlock()
	}

	t.watchersMutex.Lock()
	w, watcherExists := t.watchers[watcherId]

	if watcherExists {
		// Extract collected changes if watcher exists
		t.watchersMutex.Unlock()
		return w.extractChanges()
	} else {
		// Otherwise return full value and create a new watcher
		t.watchers[watcherId] = newWatcher(watcherId)
		t.watchersMutex.Unlock()
		return t.GetValue()
	}
}

func (t *Tree) AddTypes(typesStr string) error {
	types, err := RegisteredTypes.GetTypesFromStr(typesStr)
	if err != nil {
		return err
	}
	for _, ot := range types {
		t.objectTypes[ot.Name] = ot
	}
	return nil
}

func (t *Tree) AddType(newObjectFunc NewObjectFunc, name string) {
	t.objectTypes[name] = NewObjectType(newObjectFunc, name)
}

func (t *Tree) AddPlugin(pluginFilename string) error {
	newTypes, err := NewObjectTypesFromPlugin(pluginFilename)
	if err != nil {
		return err
	}
	for _, ot := range newTypes {
		t.objectTypes[ot.Name] = ot
	}
	return nil
}

func (t *Tree) GetType(name string) *ObjectType {
	if val, ok := t.objectTypes[name]; ok {
		return val
	}
	return nil
}

func (t *Tree) Root() *node {
	return t.rootNode
}

func (t *Tree) IsModified() bool {
	return t.modified
}

func (t *Tree) ResetModified() {
	t.modified = false
}

func (t *Tree) SetName(path string) {
	t.name = path
}

func (t *Tree) GetName() string {
	return t.name
}

func (t *Tree) SetDatasource(datasource Datasource) {
	t.datasource = datasource
}

func (t *Tree) GetDatasource() Datasource {
	return t.datasource
}
