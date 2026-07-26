#!/usr/bin/env bash
#
# Собирает релизные архивы wend под все поддерживаемые платформы.
#
#   ./scripts/build-release.sh [версия]
#
# Версия по умолчанию берётся из git describe. Результат кладётся в dist/:
# по архиву на платформу плюс SHA256SUMS.
#
# Скрипт лежит отдельно от workflow намеренно: релиз можно собрать и
# проверить локально, не дожидаясь прогона в CI и не гадая, что именно
# соберёт GitHub.
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
OUT="${OUT:-dist}"

PLATFORMS=(
	linux/amd64
	linux/arm64
	darwin/amd64
	darwin/arm64
	windows/amd64
	windows/arm64
)

rm -rf "$OUT"
mkdir -p "$OUT"
outdir="$(cd "$OUT" && pwd)"

# Под Windows принято отдавать zip. Утилита zip есть не везде (на голой Arch,
# например, нет), поэтому предусмотрен фолбэк на модуль zipfile из python3 —
# иначе скрипт собирался бы только в CI, а локально падал на середине.
make_zip() {
	local dest="$1"
	shift
	if command -v zip >/dev/null 2>&1; then
		zip -q "$dest" "$@"
	elif command -v python3 >/dev/null 2>&1; then
		python3 -m zipfile -c "$dest" "$@"
	else
		echo "нужен zip или python3 для сборки архива под windows" >&2
		exit 1
	fi
}

for platform in "${PLATFORMS[@]}"; do
	goos="${platform%/*}"
	goarch="${platform#*/}"

	binary="wend"
	if [ "$goos" = "windows" ]; then
		binary="wend.exe"
	fi

	stage="$(mktemp -d)"

	# CGO_ENABLED=0 — бинарь получается статическим и не зависит от версии
	# glibc целевой системы; -trimpath убирает из него абсолютные пути
	# сборочной машины, чтобы сборка была воспроизводимой.
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
		go build -trimpath \
		-ldflags "-s -w -X main.version=${VERSION}" \
		-o "$stage/$binary" ./cmd/wend

	cp LICENSE README.md "$stage/"

	# Файлы перечисляются явно, а не архивируется каталог целиком: иначе
	# в архиве появляется лишний уровень "./" и мусор вроде .DS_Store.
	archive="wend_${VERSION}_${goos}_${goarch}"
	if [ "$goos" = "windows" ]; then
		(cd "$stage" && make_zip "$outdir/${archive}.zip" "$binary" LICENSE README.md)
	else
		tar -czf "$outdir/${archive}.tar.gz" -C "$stage" "$binary" LICENSE README.md
	fi

	# Рядом с архивом кладётся и сам бинарь, без упаковки: со страницы
	# релиза его можно скачать и запустить сразу. Архив остаётся — он втрое
	# меньше и несёт LICENSE с README.
	#
	# В имени голого бинаря намеренно НЕТ версии: только так работает
	# постоянная ссылка вида
	#   /releases/latest/download/wend_linux_amd64
	# — GitHub требует в ней точное имя ассета, и с версией внутри она
	# ломалась бы на каждом релизе.
	bare="wend_${goos}_${goarch}"
	if [ "$goos" = "windows" ]; then
		bare="${bare}.exe"
	fi
	cp "$stage/$binary" "$outdir/$bare"
	chmod +x "$outdir/$bare"

	rm -rf "$stage"
	echo "собрано: ${archive}"
done

# Глоб раскрывается до создания файла, поэтому SHA256SUMS не попадает
# в собственный список.
(cd "$outdir" && sha256sum -- * > SHA256SUMS)
echo
echo "готово, артефакты в ${OUT}/:"
ls -1 "$outdir"
