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
// Install takes a context so a long-running install can be canceled: the CLI
// binds it to SIGINT. Modules must pass it
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
	Src    string
	Dst    string
	Policy LinkPolicy
}

// LinkPolicy says how a managed file reaches its destination.
type LinkPolicy int

const (
	// PolicyLink symlinks the file. The default.
	PolicyLink LinkPolicy = iota
	// PolicySeed is for a file its program rewrites, or one with no include
	// for local changes. With packaged content it is copied into place and
	// replaced on upgrade only while unchanged (see SeedFile). A clone
	// install links it like any other file, so editing it edits the clone.
	PolicySeed
)

func (l LinkPair) seeded() bool { return l.Policy == PolicySeed && UsingPackagedContent() }

// Apply puts the file in place by its policy.
func (l LinkPair) Apply() error {
	if l.seeded() {
		return SeedFile(l.Src, l.Dst)
	}
	return LinkFile(l.Src, l.Dst)
}

// Remove takes the file away by its policy, leaving anything that is not
// dfinstall's: a link elsewhere, a user's file, a seeded copy they changed.
func (l LinkPair) Remove() error {
	if l.seeded() {
		return UnseedFile(l.Src, l.Dst)
	}
	return UnlinkFile(l.Src, l.Dst)
}

// Check reports the file in CheckLink's terms, by its policy.
func (l LinkPair) Check() string {
	if l.seeded() {
		return CheckSeed(l.Dst)
	}
	return CheckLink(l.Src, l.Dst)
}

// LinkSet is a module's complete set of managed files.
//
// Declare it once and derive Install/Uninstall/Status from it. These three
// previously each restated the same paths, so changing a path in two of them
// left the third silently disagreeing — Status reporting on links Install no
// longer creates, or Uninstall missing one.
type LinkSet []LinkPair

// Apply puts every file in the set in place. LinkFile and SeedFile create
// missing parent directories, so callers need no EnsureDir of their own.
func (ls LinkSet) Apply() error {
	for _, l := range ls {
		if err := l.Apply(); err != nil {
			return err
		}
	}
	return nil
}

// Remove takes away every file in the set that is still dfinstall's.
func (ls LinkSet) Remove() error {
	for _, l := range ls {
		if err := l.Remove(); err != nil {
			return err
		}
	}
	return nil
}

// Status counts how many of the set's files are correctly in place.
func (ls LinkSet) Status(name string) ModuleStatus {
	s := ModuleStatus{Name: name}
	for _, l := range ls {
		if l.Check() == "ok" {
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
