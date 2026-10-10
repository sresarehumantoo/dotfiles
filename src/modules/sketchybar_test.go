package modules

import (
	"os"
	"slices"
	"testing"
)

// Links() is the only place the plugins are listed, and it can only be
// exercised on a Mac; a new plugin missing from it would never be linked.
func TestSketchybarPluginsListed(t *testing.T) {
	entries, err := os.ReadDir("../../config/sketchybar/plugins")
	if err != nil {
		t.Fatal(err)
	}
	var onDisk []string
	for _, e := range entries {
		onDisk = append(onDisk, e.Name())
	}
	listed := slices.Clone(sketchybarPlugins)
	slices.Sort(onDisk)
	slices.Sort(listed)
	if !slices.Equal(onDisk, listed) {
		t.Errorf("sketchybarPlugins = %v, config/sketchybar/plugins has %v", listed, onDisk)
	}
}
