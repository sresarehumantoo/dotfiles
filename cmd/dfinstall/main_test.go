package main

import (
	"testing"

	"github.com/sresarehumantoo/dotfiles/v2/internal/modules"
)

func TestResolveTargets(t *testing.T) {
	modules.RegisterAllModules()

	all, mods, err := resolveTargets([]string{"docs", "htop", "docs"})
	if err != nil || all {
		t.Fatalf("resolveTargets: all=%v err=%v", all, err)
	}
	var names []string
	for _, m := range mods {
		names = append(names, m.Name())
	}
	if len(names) != 2 || names[0] != "htop" || names[1] != "docs" {
		t.Errorf("want [htop docs] in registration order, deduplicated; got %v", names)
	}

	if all, _, err := resolveTargets([]string{"all"}); err != nil || !all {
		t.Errorf("'all' alone: all=%v err=%v", all, err)
	}
	if _, _, err := resolveTargets([]string{"all", "htop"}); err == nil {
		t.Error("'all' combined with a module name must be refused")
	}
	if _, _, err := resolveTargets([]string{"htop", "nosuch"}); err == nil {
		t.Error("an unknown module must be refused")
	}
}
