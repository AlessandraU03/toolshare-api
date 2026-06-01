#!/bin/bash
# =============================================================================
# setup.sh — Script de configuración inicial del proyecto
# Ejecutar una sola vez después de clonar el repositorio.
# =============================================================================

set -e  # Salir inmediatamente si algún comando falla

echo "=================================================="
echo " 🔧 Tool Inventory API — Setup Inicial"
echo "=================================================="

# 1. Copiar el archivo de variables de entorno
if [ ! -f .env ]; then
    cp .env.example .env
    echo "✅ Archivo .env creado desde .env.example"
    echo "   ⚠️  Edita .env con tus credenciales de PostgreSQL antes de continuar"
else
    echo "ℹ️  El archivo .env ya existe, no se sobreescribe"
fi

# 2. Inicializar módulo Go (solo si no existe go.sum)
if [ ! -f go.sum ]; then
    echo ""
    echo "📦 Descargando dependencias de Go..."
    go mod tidy
    echo "✅ Dependencias instaladas"
else
    echo "ℹ️  Las dependencias ya están instaladas"
fi

echo ""
echo "=================================================="
echo " ✅ Setup completado"
echo ""
echo " Próximos pasos:"
echo "   1. Edita .env con tu DATABASE_URL"
echo "   2. Aplica la migración SQL:"
echo "      psql -U postgres -d tool_inventory -f migrations/001_initial_schema.sql"
echo "   3. Levanta el servidor:"
echo "      go run ./cmd/server/main.go"
echo "=================================================="
