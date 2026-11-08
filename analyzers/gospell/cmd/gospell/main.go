// Command gospell reports words spelled the way Go does not spell them.
package main

import (
	"lesiw.io/tools/analyzers/gospell"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(gospell.Analyzer) }
