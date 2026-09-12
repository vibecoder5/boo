package docs

import _ "embed"

// UserGuide — исходный текст docs/user-guide.md, встроенный в читалку.
//
//go:embed user-guide.md
var UserGuide []byte
