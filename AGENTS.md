# Память проекта boo

Локальная офлайн-читалка (Go 1.24 + встроенный `web/`). UI и ошибки — на русском. Нет аккаунтов, облака и внешних API.

## Карта кода

- `main.go` — флаги `-port` (7474), `-web`, `-no-open`, `-demo`; `//go:embed all:web`; старт сервера; Windows-окно или браузер.
- `internal/open` — EPUB / FB2 / FB2.zip / TXT / MD → `*epub.Book`.
- `internal/epub` — модель книги, санитайз HTML, поиск, демо `Sample()`.
- `internal/fb2`, `internal/txt` — конвертеры в ту же модель.
- `internal/server` — HTTP только с loopback; UI с `/` и `/static/`.
- `internal/store` — `%AppData%/boo/state.json` (+ `library/`, `covers/`, `dictionaries/`).
- `internal/dict` — DSL, XDXF, JSON, строки; lookup по слову.
- `internal/desktop` — WebView2 на Windows; `Supported() == false` иначе.
- `web/app.js`, `web/index.html`, `web/styles.css` — без бандлера.

Ключ книги: `id:<identifier>` или `file:<sha256>`. Демо: `id:urn:uuid:boo-demo`. Руководство: `id:urn:uuid:boo-guide` (`docs/user-guide.md`, `POST /api/guide`).

## Инварианты

- Не слушать не-loopback и не ходить в сеть за данными пользователя.
- Форматы сходятся в `epub.Book`; HTML глав — через `sanitizeChapter`.
- `/res` отдаёт только image/* и font/*.
- UI общий (`store.Data.UI`, словари); полка, прогресс, закладки, заметки, todos, history, undo, списки — в текущем `Workspace`.
- Импорт ZIP (`boo-library` v1) заменяет библиотеку целиком.
- Undo: книга, закладка, заметка — не выделения.
- Сообщения API — русские фразы, не английские коды для пользователя.

## Как менять

- Новое поле persist — миграция в `store.Open` / `migrateWorkspaces`, тест в `store_test.go` или `transfer_test.go`.
- Новый маршрут — в `Server.Handler()`, доступ только localhost, ответ JSON UTF-8.
- Фронт: ванильный JS, состояние с `GET /api/state`, не вводить фреймворк.
- После правок парсеров, store, server: `go test ./...`.
- Не раздувать зависимости. Окно десктопа не портировать на другие ОС без явной задачи.
- Заметное изменение поведения, API или persist — поднять SemVer в `CHANGELOG.md` (правило `.cursor/rules/versioning.mdc`).

Подробности для людей: `README.md`, `docs/user-guide.md`, `CHANGELOG.md`.
