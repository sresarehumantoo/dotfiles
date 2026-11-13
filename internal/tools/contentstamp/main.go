// Command contentstamp writes the content stamp for a checkout's config/ tree,
// for the release pipeline: go run ./internal/tools/contentstamp -version X.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sresarehumantoo/dotfiles/v3/internal/core"
)

func main() {
	root := flag.String("root", ".", "content root holding config/")
	version := flag.String("version", "", "release version to record (required)")
	out := flag.String("o", "", "output path (default <root>/"+core.ContentStampName+")")
	flag.Parse()

	if *out == "" {
		*out = filepath.Join(*root, core.ContentStampName)
	}
	st, err := core.GenerateContentStamp(*root, *version)
	if err == nil {
		err = core.WriteContentStamp(*out, st)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "contentstamp:", err)
		os.Exit(1)
	}
}
