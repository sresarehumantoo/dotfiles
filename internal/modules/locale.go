package modules

import (
	"context"
	"strings"

	"github.com/sresarehumantoo/dotfiles/v2/internal/core"
)

type LocaleModule struct{}

func (LocaleModule) Name() string { return "locale" }

// localeGenerated checks if the given locale is available on the system.
func localeGenerated(name string) bool {
	out, err := runProbe(context.Background(), "locale", "-a")
	if err != nil {
		return false
	}
	// glibc prints "en_US.utf8", macOS "en_US.UTF-8"
	normalize := func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "-", "")
	}
	target := normalize(name)
	for _, line := range strings.Split(string(out), "\n") {
		if normalize(line) == target {
			return true
		}
	}
	return false
}

// enUSCommand generates en_US.UTF-8 on Debian and Ubuntu. Printed, not run:
// shells fall back to C.UTF-8 without it, so it is a preference for en_US date
// formats and sorting, not something worth root on every machine.
const enUSCommand = "sudo apt install locales && sudo sed -i 's/^# *en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && sudo locale-gen"

// Install changes nothing on the system. config/shell/zsh/locale.zsh picks
// en_US.UTF-8 when it is generated and C.UTF-8 otherwise; this only reports
// which one shells will get.
func (LocaleModule) Install(ctx context.Context) error {
	switch {
	case localeGenerated("en_US.UTF-8"):
		core.Ok("Locale en_US.UTF-8 available")
	case core.IsMac():
		core.Warn("en_US.UTF-8 is missing from `locale -a`, and macOS has no locale-gen to add it")
	case localeGenerated("C.UTF-8"):
		core.Notice("en_US.UTF-8 is not generated, so shells use C.UTF-8 (UTF-8 with C sorting and date formats). For en_US: %s", enUSCommand)
	default:
		core.AlwaysWarn("No UTF-8 locale is available. Generate en_US.UTF-8 with: %s", enUSCommand)
	}
	return nil
}

func (LocaleModule) Status() core.ModuleStatus {
	s := core.ModuleStatus{Name: "locale"}
	switch {
	case localeGenerated("en_US.UTF-8"):
		s.Linked++
		s.Extra = "en_US.UTF-8"
	case localeGenerated("C.UTF-8"):
		s.Linked++
		s.Extra = "C.UTF-8 (en_US.UTF-8 not generated)"
	default:
		s.Missing++
		s.Extra = "no UTF-8 locale"
	}
	return s
}
