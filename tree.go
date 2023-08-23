package forjitree

type Tree struct {
	objectTypes map[string]*ObjectType
	rootNode    *node
	created     bool
}

func New() *Tree {
	t := &Tree{
		objectTypes: make(map[string]*ObjectType),
		created:     false,
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

func (t *Tree) DestroyObjects() {
	t.rootNode.destroyObject(true)
}

func (t *Tree) GetValue() any {
	return t.rootNode.getValue()
}

func (t *Tree) Set(data any) {
	modifiedNodes := t.rootNode.patch(data)

	createdObjects := []*node{}
	for i := len(modifiedNodes) - 1; i >= 0; i-- {
		if modifiedNodes[i].synchronize() {
			createdObjects = append(createdObjects, modifiedNodes[i])
		}
	}

	for i := len(createdObjects) - 1; i >= 0; i-- {
		createdObjects[i].obj.CreatedChildren()
	}

	// Call CreatedTree if the tree has already been created
	if t.created {
		for i := 0; i < len(createdObjects); i++ {
			createdObjects[i].obj.CreatedTree()
		}
	}
}

func (t *Tree) AddType(newObjectFunc NewObjectFunc, name string) {
	t.objectTypes[name] = NewObjectType(newObjectFunc, name)
}

func (t *Tree) AddPlugin(pluginFilename string) {
	newTypes := NewObjectTypesFromPlugin(pluginFilename)
	for _, ot := range newTypes {
		t.objectTypes[ot.Name] = ot
	}
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
