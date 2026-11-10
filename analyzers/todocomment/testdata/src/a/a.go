package a

// TODO
func bare() {} // want `note with no detail`

// FIXME: fix this
func withDetail() {}

// TODO(team): finish
func withOwner() {}

// A comment that mentions a TODO in passing is fine.
func mention() {}

// BUG
func bug() {} // want `note with no detail`

// BUG()
func emptyParens() {} // want `note with no detail`
