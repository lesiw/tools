package reflow

import "lesiw.io/tools/analyzers/linelen"

// The commands give the width rules the house limit; the tests run what the
// commands run.
func init() { linelen.Config.Len = 79 }
