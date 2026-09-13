---
name: create-release
description: >-
  Creates a boo GitHub release: SemVer from CHANGELOG.md, installer build,
  annotated tag vX.Y.Z, and gh release with dist/boo-setup.exe. Use when the
  user asks to release, publish, cut a version, tag, выпустить, сделать релиз,
  or create a GitHub Release.
---

# Релиз boo

Просьба «сделай релиз» — разрешение закоммитить релизные правки, поставить тег, запушить и создать GitHub Release. Не делать force-push, не переписывать существующий тег без явной просьбы.

## Версия

Канон — последний заголовок `## [X.Y.Z]` в `CHANGELOG.md` (правило `.cursor/rules/versioning.mdc`). Другого файла с номером нет.

1. Прочитать `CHANGELOG.md`, `git status`, `git log -10 --oneline`, `git tag -l`.
2. Если пользователь назвал номер — использовать его. Иначе:
   - в `[Unreleased]` есть пункты → поднять PATCH / MINOR / MAJOR по правилу версий;
   - `[Unreleased]` пуст → релиз текущего `X.Y.Z`, если тега `vX.Y.Z` ещё нет.
3. Перенести пункты из `[Unreleased]` в `## [X.Y.Z] — YYYY-MM-DD` (дата из контекста сессии). Секцию `## [Unreleased]` оставить пустой сверху.
4. Синхронизировать запасной номер: `var Version` в `internal/install/install.go`. Сборка всё равно берёт номер из CHANGELOG через `-ldflags`.

Текст в журнале — для человека, по-русски, не список файлов.

Не поднимать версию, если в релизе только `.cursor/`, `AGENTS.md` или скиллы.

## Проверки

Рабочее дерево: либо чистое, либо только то, что входит в этот релиз. Посторонние правки — спросить.

```powershell
go test ./...
powershell -File scripts/build-installer.ps1
```

Оба должны пройти. Артефакты: `dist/boo.exe`, `dist/boo-setup.exe`. Каталог `dist/` в git не добавлять (`*.exe` и `dist/` в `.gitignore`).

## Коммит и тег

Если есть незакоммиченные релизные файлы — один коммит, сообщение `Release X.Y.Z.`

```powershell
git add CHANGELOG.md internal/install/install.go
# плюс остальные файлы этого релиза, не dist/
git commit -m "Release X.Y.Z."
git tag -a "vX.Y.Z" -m "boo X.Y.Z"
```

Тег только после зелёных тестов и успешной сборки setup. Если `vX.Y.Z` уже есть — остановиться.

## Публикация

```powershell
git push origin HEAD
git push origin "vX.Y.Z"
```

Новости релиза — секция этой версии из `CHANGELOG.md` (без заголовка `# Changelog` и без `[Unreleased]`).

```powershell
gh release create "vX.Y.Z" "dist/boo-setup.exe" --title "boo X.Y.Z" --notes "..."
```

К релизу прилагать `dist/boo-setup.exe`. `boo.exe` отдельно не класть: читатель ставит через setup.

Если `gh` не авторизован — локальный тег оставить, пуш/релиз не имитировать, написать что осталось сделать вручную.

## После

Вернуть URL релиза (`https://github.com/vibecoder5/boo/releases/tag/vX.Y.Z`) и номер версии. Не коммитить `dist/`, `*.syso`, `*.exe` в корне.
