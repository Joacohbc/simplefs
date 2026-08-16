#!/usr/bin/env bash
set -e

echo "📦 Compilando assets con pnpm y Tailwind CSS..."
pnpm run build

PORT=${PORT:-8080}
echo "🌟 Iniciando SimpleFS..."
./simplefs "$@"

