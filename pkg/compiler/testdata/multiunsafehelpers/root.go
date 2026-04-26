package multiunsafehelpers

import "unsafe"

func Entry() {
	var ptr unsafe.Pointer
	first(ptr)
	second(ptr)
}

func first(ptr unsafe.Pointer) unsafe.Pointer {
	return ptr
}

func second(ptr unsafe.Pointer) unsafe.Pointer {
	return ptr
}
