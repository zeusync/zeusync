package systems

type System interface {
	Name() string
	Update(deltaTime float64, world World) error
	Initialize(world World) error
	Shutdown() error
	Dependencies() []string
}

type SystemManager interface {
	RegisterSystem(System) error
	UnregisterSystem(string) error
	UpdateSystems(deltaTime float64) error
	UpdateSystem(deltaTime float64, name string) error
	EnableSystem(string) error
	DisableSystem(string) error
	GetSystem(string) (System, error)
	ListSystems() []System
}

type World interface{}
