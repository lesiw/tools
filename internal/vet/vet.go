// Package vet names the lesiw.io analyzer suite.
package vet

import "golang.org/x/tools/go/analysis"

// Analyzers returns the suite: the x/tools passes and the lesiw.io analyzers.
func Analyzers() []*analysis.Analyzer { return analyzers }
