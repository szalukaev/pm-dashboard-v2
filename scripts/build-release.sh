#!/bin/sh
# Собирает поставку для клиента: release/pm-dashboard-<версия>.tar.gz.
# В архиве готовые образы (без исходного кода), docker-compose.yml,
# .env.example, скрипт отпечатка сервера и инструкция по установке.
#
# Запуск на машине с Docker, из корня репозитория:
#     sh scripts/build-release.sh 3.0.1
# Модуль лицензирования должен лежать в backend/_private/pmd-license.
set -eu
cd "$(dirname "$0")/.."

VERSION=${1:?укажите версию: sh scripts/build-release.sh 3.0.1}
NAME=pm-dashboard-$VERSION
OUT=release/$NAME
PLATFORM=linux/amd64
EXTRA_IMAGES="redis:7-alpine alpine:3.19"

[ -f backend/_private/pmd-license/go.mod ] || { echo "нет модуля backend/_private/pmd-license" >&2; exit 1; }

echo "== Образы $VERSION"
docker build --platform $PLATFORM -f Dockerfile.backend -t pm-dashboard-backend:$VERSION backend
docker build --platform $PLATFORM -f Dockerfile.frontend -t pm-dashboard-frontend:$VERSION frontend
for image in $EXTRA_IMAGES; do
    docker pull --platform $PLATFORM "$image"
done

echo "== Архив"
rm -rf "$OUT"
mkdir -p "$OUT/scripts"
docker save pm-dashboard-backend:$VERSION pm-dashboard-frontend:$VERSION $EXTRA_IMAGES | gzip > "$OUT/images.tar.gz"
sed "s/__VERSION__/$VERSION/g" docker-compose.release.yml > "$OUT/docker-compose.yml"
cp .env.example "$OUT/.env.example"
cp scripts/collect-hwid.sh "$OUT/scripts/"
sed "s/__VERSION__/$VERSION/g" scripts/INSTALL.release.md > "$OUT/INSTALL.md"
tar czf "release/$NAME.tar.gz" -C release "$NAME"
rm -rf "$OUT"

# Образы на сборочной машине больше не нужны: место занимает только архив
docker image rm pm-dashboard-backend:$VERSION pm-dashboard-frontend:$VERSION >/dev/null 2>&1 || true

echo "Готово: release/$NAME.tar.gz ($(du -h "release/$NAME.tar.gz" | cut -f1))"
