package a

import "reflect"

func prodUse(a, b any) bool { return reflect.DeepEqual(a, b) }
