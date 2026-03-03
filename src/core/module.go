package core

// ModuleStatus represents the install status of a module.
type ModuleStatus struct {
	Name    string
	Linked  int
	Missing int
	Extra   string // additional info
}

// Module is the interface every install module implements.
type Module interface {
	Name() string
	Install() error
	Status() ModuleStatus
}

// Uninstaller is an optional interface for modules that support removal.
type Uninstaller interface {
	Uninstall() error
}

// LinkPair describes a single source → destination symlink.
type LinkPair struct {
	Src string
	Dst string
}

// LinkExporter is an optional interface for modules that manage symlinks.
type LinkExporter interface {
	Links() []LinkPair
}

var modules []Module
var moduleMap map[string]Module

func init() {
	moduleMap = make(map[string]Module)
}

// RegisterModule adds a module to the ordered registry.
func RegisterModule(m Module) {
	modules = append(modules, m)
	moduleMap[m.Name()] = m
}

// GetModule returns a module by name and whether it was found.
func GetModule(name string) (Module, bool) {
	m, ok := moduleMap[name]
	return m, ok
}

// AllModules returns all registered modules in order.
func AllModules() []Module {
	return modules
}

// ModuleNames returns the names of all registered modules in order.
func ModuleNames() []string {
	names := make([]string, len(modules))
	for i, m := range modules {
		names[i] = m.Name()
	}
	return names
}
