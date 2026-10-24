package core

import (
	"fmt"

	"github.com/fatih/color"
)

// PrintBanner prints the dfinstall ASCII art banner in cyan+bold.
func PrintBanner() {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	fmt.Println(cyan(`     _  __ _         _        _ _`))
	fmt.Println(cyan(`  __| |/ _(_)_ _  __| |_ __ _| | |`))
	fmt.Println(cyan(` / _` + "`" + `|  _| | ' \(_-<  _/ _` + "`" + `| | | |`))
	fmt.Println(cyan(` \__,_|_| |_|_||_/__/\__\__,_|_|_|`))
	fmt.Println()
	fmt.Println(green(`		by: SREsAreHumanToo`))
	fmt.Println()
}
