package modules

import (
	"fmt"

	"github.com/sresarehumantoo/dotfiles/src/core"
)

// RegisterAllModules registers every module in the correct install order.
// Called once from main — avoids per-file init() non-determinism.
func RegisterAllModules() {
	core.RegisterModule(&LocaleModule{})
	core.RegisterModule(&PackagesModule{})
	core.RegisterModule(&ExtrasModule{})
	core.RegisterModule(&ToolkitModule{})
	core.RegisterModule(&DeltaModule{})
	core.RegisterModule(&FontsModule{})
	core.RegisterModule(&OmzModule{})
	core.RegisterModule(&ShellModule{})
	core.RegisterModule(&DevtoolsModule{})
	core.RegisterModule(&GitModule{})
	core.RegisterModule(&NvimModule{})
	core.RegisterModule(&TmuxModule{})
	core.RegisterModule(&KonsoleModule{})
	core.RegisterModule(&GhosttyModule{})
	core.RegisterModule(&HtopModule{})
	core.RegisterModule(&WslModule{})
	core.RegisterModule(&VMGuestModule{})
	core.RegisterModule(&DefaultShellModule{})
}

// ValidModuleNames returns a formatted list of valid module names for error messages.
func ValidModuleNames() string {
	s := ""
	for i, n := range core.ModuleNames() {
		if i > 0 {
			s += ", "
		}
		s += n
	}
	return fmt.Sprintf("valid modules: %s", s)
}
