package unsafeedge

import "unsafe"

func Entry(ptr unsafe.Pointer) unsafe.Pointer {
	return ptr
}
