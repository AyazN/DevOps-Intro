package main

import "golang.org/x/text/language"

// demoReach exists only to make the vulnerable symbol in
// golang.org/x/text reachable from main, so govulncheck reports
// GO-2022-1059 at symbol level during the red-CI demo.
// Remove after the demo.
func demoReach() {
	_, _, _ = language.ParseAcceptLanguage("en-US,en;q=0.9")
}