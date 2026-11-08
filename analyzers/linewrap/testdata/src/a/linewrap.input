package a

import "fmt"

func longCall(ctx, arg int) error { return nil }

func wrappedCall(ctx, arg int) {
	if err := longCall( // want "if header spans 3 lines, exceeds the 1-line limit"
		ctx, arg,
	); err != nil {
		fmt.Println(err)
	}
}

func wrappedRange() {
	for k, v := range map[string]string{ // want "for header spans 3 lines, exceeds the 1-line limit"
		"a": "1",
	} {
		fmt.Println(k, v)
	}
}

func cleanFor() {
	for i := 0; i < 10; i++ {
		fmt.Println(i)
	}
}
