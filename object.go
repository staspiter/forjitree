package tree

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
