package forjitree

type Datasource interface {
	GetNode() Node

	Connect() error
	Disconnect() error

	Get(query any) (any, error)
	Set(query any) (map[string][]int, error)
	Delete(query any) (map[string][]int, error)
	Clear() error
	Watch(query any, watcherId string) (any, error)
}

type Schema interface {
	GetNode() Node

	DefaultDatasource() Datasource
	Types() string
}

type Action interface {
	GetNode() Node

	Call(schema Schema) error
}
