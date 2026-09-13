package install

import (
	"encoding/binary"
	"errors"
)

const (
	payloadMagic   = "BOOINST1"
	payloadTrailer = 8 + 8 + 8 // magic + uninstaller size + app size
)

// Bundle — содержимое setup-файла: читалка и маленький деинсталлятор.
type Bundle struct {
	App         []byte
	Uninstaller []byte
}

var (
	errPayloadTruncated = errors.New("файл установки обрезан")
	errPayloadMagic     = errors.New("нет вложения: соберите через scripts/build-installer.ps1 или укажите -bin")
	errPayloadSize      = errors.New("вложение повреждено")
)

// Pack дописывает Bundle в конец host (обычно сам setup.exe).
func Pack(host []byte, b Bundle) []byte {
	out := make([]byte, 0, len(host)+len(b.Uninstaller)+len(b.App)+payloadTrailer)
	out = append(out, host...)
	out = append(out, b.Uninstaller...)
	out = append(out, b.App...)
	var tail [payloadTrailer]byte
	copy(tail[:8], payloadMagic)
	binary.LittleEndian.PutUint64(tail[8:16], uint64(len(b.Uninstaller)))
	binary.LittleEndian.PutUint64(tail[16:24], uint64(len(b.App)))
	return append(out, tail[:]...)
}

// Unpack читает Bundle из конца файла. ok == false, если это обычный exe без вложения.
func Unpack(file []byte) (Bundle, error) {
	if len(file) < payloadTrailer {
		return Bundle{}, errPayloadTruncated
	}
	tail := file[len(file)-payloadTrailer:]
	if string(tail[:8]) != payloadMagic {
		return Bundle{}, errPayloadMagic
	}
	uninstLen := binary.LittleEndian.Uint64(tail[8:16])
	appLen := binary.LittleEndian.Uint64(tail[16:24])
	need := int(uninstLen) + int(appLen) + payloadTrailer
	if uninstLen > uint64(len(file)) || appLen > uint64(len(file)) || need > len(file) {
		return Bundle{}, errPayloadSize
	}
	start := len(file) - need
	uninst := file[start : start+int(uninstLen)]
	app := file[start+int(uninstLen) : start+int(uninstLen)+int(appLen)]
	return Bundle{App: append([]byte(nil), app...), Uninstaller: append([]byte(nil), uninst...)}, nil
}
