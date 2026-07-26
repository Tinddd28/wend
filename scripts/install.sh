#!/bin/sh
#
# Установка wend из релизных сборок GitHub.
#
#   curl -fsSL https://raw.githubusercontent.com/Tinddd28/wend/main/scripts/install.sh | sh
#
# Переменные окружения:
#   VERSION   тег релиза (по умолчанию последний стабильный; nightly —
#             свежая сборка из main)
#   PREFIX    каталог установки (по умолчанию /usr/local/bin)
#
# Флаги:
#   --dry-run   показать, что будет скачано и куда установлено, и выйти
#
# Скрипт на POSIX sh, а не bash: на многих минимальных системах (Alpine,
# контейнеры) bash не установлен, а установщик должен работать именно там.
set -eu

REPO="Tinddd28/wend"
VERSION="${VERSION:-latest}"
PREFIX="${PREFIX:-/usr/local/bin}"
DRY_RUN=0

for arg in "$@"; do
	case "$arg" in
	--dry-run) DRY_RUN=1 ;;
	-h | --help)
		sed -n '2,20p' "$0" | sed 's/^#//;s/^ //'
		exit 0
		;;
	*)
		echo "неизвестный аргумент: $arg" >&2
		exit 2
		;;
	esac
done

die() {
	echo "wend: $*" >&2
	exit 1
}

# --- сеть -------------------------------------------------------------------

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL "$1"; }
	download() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -qO- "$1"; }
	download() { wget -qO "$2" "$1"; }
else
	die "нужен curl или wget"
fi

# --- платформа --------------------------------------------------------------

case "$(uname -s)" in
Linux) os="linux" ;;
Darwin) os="darwin" ;;
*)
	die "платформа $(uname -s) не поддерживается этим скриптом.
Готовые сборки, в том числе под Windows: https://github.com/$REPO/releases"
	;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch="amd64" ;;
aarch64 | arm64) arch="arm64" ;;
*) die "архитектура $(uname -m) не поддерживается; соберите из исходников: go install github.com/$REPO/cmd/wend@latest" ;;
esac

# --- версия -----------------------------------------------------------------

if [ "$VERSION" = "latest" ]; then
	# Тег последнего стабильного релиза берётся из API. jq намеренно не
	# используется: он есть далеко не везде, а поле нужно ровно одно.
	VERSION="$(
		fetch "https://api.github.com/repos/$REPO/releases/latest" |
			grep -m1 '"tag_name"' |
			sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/'
	)" || die "не удалось определить последнюю версию"

	[ -n "$VERSION" ] || die "стабильных релизов пока нет.
Свежую сборку из main можно поставить так:  VERSION=nightly $0"
fi

archive="wend_${VERSION}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$VERSION"

echo "wend $VERSION  ($os/$arch)"
echo "  архив:    $base/$archive"
echo "  установка: $PREFIX/wend"

if [ "$DRY_RUN" = "1" ]; then
	echo "(--dry-run: ничего не скачано)"
	exit 0
fi

# --- загрузка и проверка ----------------------------------------------------

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

download "$base/$archive" "$tmp/$archive" || die "не удалось скачать $archive"

if command -v sha256sum >/dev/null 2>&1; then
	sha256_of() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
	sha256_of() { shasum -a 256 "$1" | awk '{print $1}'; }
else
	sha256_of() { echo ""; }
fi

# Контрольная сумма проверяется по умолчанию и её несовпадение — жёсткая
# ошибка: скрипт кладёт исполняемый файл в PATH, тихо доверять скачанному
# нельзя. SKIP_CHECKSUM=1 оставлен для систем совсем без sha256-утилит.
if [ "${SKIP_CHECKSUM:-0}" != "1" ]; then
	actual="$(sha256_of "$tmp/$archive")"
	[ -n "$actual" ] || die "не найдено ни sha256sum, ни shasum.
Установите одну из них либо повторите с SKIP_CHECKSUM=1 (на свой риск)."

	download "$base/SHA256SUMS" "$tmp/SHA256SUMS" || die "не удалось скачать SHA256SUMS"
	expected="$(grep " [*]\{0,1\}$archive\$" "$tmp/SHA256SUMS" | awk '{print $1}')"
	[ -n "$expected" ] || die "в SHA256SUMS нет записи для $archive"

	if [ "$actual" != "$expected" ]; then
		die "контрольная сумма не совпала!
  ожидалось: $expected
  получено:  $actual"
	fi
	echo "  sha256:   ок"
fi

tar -xzf "$tmp/$archive" -C "$tmp" wend || die "не удалось распаковать архив"

# --- установка --------------------------------------------------------------

mkdir -p "$PREFIX" 2>/dev/null || true
if [ -w "$PREFIX" ]; then
	install -m 0755 "$tmp/wend" "$PREFIX/wend"
elif command -v sudo >/dev/null 2>&1; then
	echo "  $PREFIX недоступен на запись, повышаю права через sudo"
	sudo install -d -m 0755 "$PREFIX"
	sudo install -m 0755 "$tmp/wend" "$PREFIX/wend"
else
	die "нет прав на запись в $PREFIX и нет sudo.
Укажите свой каталог:  PREFIX=\$HOME/.local/bin $0"
fi

echo
echo "установлено: $PREFIX/wend"
if ! command -v wend >/dev/null 2>&1; then
	echo "внимание: $PREFIX отсутствует в PATH — добавьте его в свой профиль"
fi
