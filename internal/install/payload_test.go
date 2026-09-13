package install

import "testing"

func TestPackUnpack(t *testing.T) {
	host := []byte("SETUP")
	b := Bundle{App: []byte("APP-BYTES"), Uninstaller: []byte("UNINST")}
	packed := Pack(host, b)
	got, err := Unpack(packed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.App) != "APP-BYTES" || string(got.Uninstaller) != "UNINST" {
		t.Fatalf("bundle: app=%q uninst=%q", got.App, got.Uninstaller)
	}
}

func TestUnpackPlainExe(t *testing.T) {
	_, err := Unpack([]byte("this-is-a-plain-exe-without-magic"))
	if err != errPayloadMagic {
		t.Fatalf("got %v", err)
	}
}

func TestUnpackTruncated(t *testing.T) {
	_, err := Unpack([]byte("BOO"))
	if err != errPayloadTruncated {
		t.Fatalf("got %v", err)
	}
}

func TestUnpackBadSize(t *testing.T) {
	raw := Pack([]byte("H"), Bundle{App: []byte("abc"), Uninstaller: []byte("u")})
	raw[len(raw)-1] = 0xFF
	raw[len(raw)-2] = 0xFF
	_, err := Unpack(raw)
	if err != errPayloadSize {
		t.Fatalf("got %v", err)
	}
}
