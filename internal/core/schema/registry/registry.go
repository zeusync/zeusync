package registry

type TypeName string

type SchemaRegistry interface {
	RegisterType(name string, schema TypeSchema) error
	UnregisterType(name string) error
	GetType(name string) (TypeSchema, error)
	ListTypes() []string

	RegisterVersion(typeName TypeName, version string, schema TypeSchema) error
	GetVersion(typeName, version string) (TypeSchema, error)
	GetLatestVersion(typeName string) (string, TypeSchema, error)

	CreateMigration(fromVersion, toVersion string, migrator SchemaMigrator) error
	Migrate(typeName TypeName, fromVersion, toVersion string, data any) (any, error)

	GenerateCode(typeName TypeName, language string) ([]byte, error)
	GenerateSDK(language string) (map[string][]byte, error)
}

type TypeSchema interface {
	Name() string
	Version() string
	Fields() []FiledSchema
	Validate(any) error
	Default() any

	Serialize(any) ([]byte, error)
	Deserialize([]byte) (any, error)

	Metadata() map[string]any
	Documentation() string
}

type FiledSchema interface {
	Name() string
	Type() FieldType
	Required() bool
	Default() any
	Constraints() []FiledConstraint
	Metadata() map[string]any
}

type FieldType uint16

type FiledConstraint interface{}

type SchemaMigrator interface{}
