package a

import "fmt"

func f(a, b int) bool { return false }

// Each if in an else-if chain owns its own header. A wrap in one link
// is reported at that link's if keyword, not at the head of the chain.
func elseIf(a, b int) {
	if a == 0 {
		fmt.Println("zero")
	} else if f(a, // want "if header spans 2 lines, exceeds the 1-line limit"
		b) {
		fmt.Println("f")
	} else {
		fmt.Println("other")
	}
}

func elseBlock(a int) {
	if a == 0 {
		fmt.Println("zero")
	} else {
		fmt.Println("nonzero")
		fmt.Println("still nonzero")
	}
}

func bareForms(xs []int) {
Loop:
	for {
		for range xs {
			break Loop
		}
	}
}

// A label does not move the report: it lands on the for keyword.
func labeled(xs []int) {
Outer:
	for i := 0; i < len( // want "for header spans 2 lines, exceeds the 1-line limit"
		xs); i++ {
		continue Outer
	}
}

// A block comment that pushes the brace onto a later line is reported:
// the header genuinely occupies more than one line.
func blockComment(a, b int) {
	if f(a, // want "if header spans 3 lines, exceeds the 1-line limit"
		/* a long
		explanation */b) {
		fmt.Println("zero")
	}
}

// A trailing line comment sits after the brace and never affects the
// verdict, no matter how long it makes the line.
func trailingComment(a int) {
	if a == 0 { // this comment is irrelevant to the header check
		fmt.Println("zero")
	}
	for range 3 { // so is this one
		fmt.Println("tick")
	}
}

// A wrapped call in the body is fine; only headers are checked.
func bodyWrap(a, b int) {
	if a == 0 {
		fmt.Println(
			a,
			b,
		)
	}
}

// Nested wraps are each reported.
func nested(a, b int) {
	if f(a, // want "if header spans 2 lines, exceeds the 1-line limit"
		b) {
		for i := 0; i < len( // want "for header spans 2 lines, exceeds the 1-line limit"
			[]int{a}); i++ {
			fmt.Println(i)
		}
	}
}

// A bare switch, a tagged switch, and a type switch all keep their
// brace on the keyword line.
func bareSwitch(a int) {
	switch {
	case a == 0:
		fmt.Println("zero")
	}
	switch a {
	case 0:
		fmt.Println("zero")
	}
	switch x := any(a).(type) {
	case int:
		_ = x
	}
}

// case clauses are not headers. A wrapped case expression or a wrapped
// case body is never reported.
func caseWrap(a, b int) {
	switch {
	case f(a,
		b):
		fmt.Println("yes")
	case f(b,
		a):
		fmt.Println("no")
	}
}

// A wrapped tag expression pushes the brace down and is reported.
func switchTagWrap(a, b int) {
	switch f(a, // want "switch header spans 2 lines, exceeds the 1-line limit"
		b) {
	case true:
		fmt.Println("yes")
	}
}

// A wrapped init statement is reported the same way.
func switchInitWrap(a, b int) {
	switch x := f(a, // want "switch header spans 2 lines, exceeds the 1-line limit"
		b); x {
	case true:
		fmt.Println("yes")
	}
}

// A type switch with a wrapped init or a wrapped assign is reported.
func typeSwitchWrap(a, b int) {
	switch v := any( // want "switch header spans 2 lines, exceeds the 1-line limit"
		a).(type) {
	case int:
		_ = v
	}
	switch x := f(a, // want "switch header spans 2 lines, exceeds the 1-line limit"
		b); v := any(x).(type) {
	case bool:
		_ = v
	}
}

func wrappedSig(
	a int,
	b int,
) {
	fmt.Println(a, b)
}

func wrappedLit(xs []int) {
	go func(
		n int,
	) {
		fmt.Println(n)
	}(len(xs))
}
