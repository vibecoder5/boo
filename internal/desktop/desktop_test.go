package desktop

import "testing"

func TestSupported(t *testing.T) {
	if !Supported() && Run("http://127.0.0.1/", "") == nil {
		t.Fatal("unsupported platform should not succeed")
	}
}

func TestWindowTitle(t *testing.T) {
	if windowTitle("") != "boo" {
		t.Fatal("empty")
	}
	if windowTitle(" Лето ") != "Лето — boo" {
		t.Fatal(windowTitle(" Лето "))
	}
}
