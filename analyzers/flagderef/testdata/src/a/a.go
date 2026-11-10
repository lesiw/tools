package a

import "flag"

var b = *flag.Bool("b", false, "b docs") // want `flag read before Parse`

func f() {
	_ = *flag.String("s", "", "s docs") // want `flag read before Parse`
	_ = flag.Int("n", 0, "n docs")
}

var (
	count int
)

func init() { flag.IntVar(&count, "count", 0, "count docs") }

func g(p *int) int { return *p }
