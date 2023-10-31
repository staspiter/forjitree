package forjitree

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
}

type Schema interface {
	Object

	NewContext(event any) Context

	DefaultDatasource() Datasource
	GetTypes() []string
}

type Action interface {
	Object

	Call(context Context) error
}
