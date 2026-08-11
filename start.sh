#!/usr/bin/env bash
set -e

echo "🚀 Compilando SimpleFS..."
go build -buildvcs=false -o simplefs .

PORT=${PORT:-8080}
echo "🌟 Iniciando SimpleFS en http://localhost:${PORT}"
./simplefs
