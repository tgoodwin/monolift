//go:build !cgo

package sample

func CgoMode() string {
	return "cgo-off"
}
