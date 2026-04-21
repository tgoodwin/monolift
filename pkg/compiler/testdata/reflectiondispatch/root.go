package reflectiondispatch

import "reflect"

func Entry(fn func()) {
	reflect.ValueOf(fn).Call(nil)
}
