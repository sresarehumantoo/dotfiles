package core

import "context"

// ModuleStatus represents the install status of a module.
type ModuleStatus struct {
	Name    string
	Linked  int
	Missing int
	Extra   string // additional info
}

// Module is the interface every install module implements.
//
// Install takes a context so a long-running install can be cancelled: the CLI
// binds it to SIGINT, and the MCP server passes the per-request context so a
// disconnecting client doesn't leave apt running forever. Modules must pass it
// down to every subprocess they spawn.
type Module interface {
	Name() string
	Install(ctx context.Context) error
	Status() ModuleStatus
}

// Uninstaller is an optional interface for modules that support removal.
type Uninstaller interface {
	Uninstall(ctx context.Context) error
}

// LinkPair describes a single source → destination symlink.
type LinkPair struct {
	Src string
	Dst string
}

// LinkSet is a module's complete set of managed symlinks.
//
// Declare it once and derive Install/Uninstall/Status from it. These three
// previously each restated the same paths, so changing a path in two of them
// left the third silently disagreeing — Status reporting on links Install no
// longer creates, or Uninstall missing one.
type LinkSet []LinkPair

// Apply creates every link in the set. LinkFile creates missing parent
// directories, so callers need no EnsureDir of their own.
func (ls LinkSet) Apply() error {
	for _, l := range ls {
		if err := LinkFile(l.Src, l.Dst); err != nil {
			return err
		}
	}
	return nil
}

// Remove unlinks every link in the set. UnlinkFile leaves anything that isn't
// our symlink alone, so this won't delete a user's real file.
func (ls LinkSet) Remove() error {
	for _, l := range ls {
		if err := UnlinkFile(l.Src, l.Dst); err != nil {
			return err
		}
	}
	return nil
}

// Status counts how many of the set's links are correctly in place.
func (ls LinkSet) Status(name string) ModuleStatus {
	s := ModuleStatus{Name: name}
	for _, l := range ls {
		if CheckLink(l.Src, l.Dst) == "ok" {
			s.Linked++
		} else {
			s.Missing++
		}
	}
	return s
}

// LinkExporter is an optional interface for modules that manage symlinks.
// Implementing it is what makes a module's links visible to `diff` and to
// drift detection, so every module with links should.
type LinkExporter interface {
	Links() LinkSet
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
