//go:build windows

package desktop

import "testing"

func TestShowMaximizedNil(t *testing.T) {
	showMaximized(nil)
}
