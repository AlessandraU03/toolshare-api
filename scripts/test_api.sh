#!/bin/bash
# =============================================================================
# test_api.sh — Prueba completa del flujo Tool Rental API
# =============================================================================

BASE_URL="http://localhost:8080/api"
PASS=0
FAIL=0

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Verifica que la respuesta contenga el string esperado
check() {
    local name="$1"
    local expected="$2"
    local actual="$3"
    if echo "$actual" | grep -q "$expected"; then
        echo -e "  ${GREEN}✓${NC} $name"
        ((PASS++))
    else
        echo -e "  ${RED}✗${NC} $name"
        echo -e "    ${YELLOW}esperado contener:${NC} $expected"
        echo -e "    ${YELLOW}recibido:${NC} $(echo $actual | head -c 200)"
        ((FAIL++))
    fi
}

# Verifica que el HTTP status code sea el esperado
check_status() {
    local name="$1"
    local expected="$2"
    local actual="$3"
    if [ "$actual" = "$expected" ]; then
        echo -e "  ${GREEN}✓${NC} $name (HTTP $actual)"
        ((PASS++))
    else
        echo -e "  ${RED}✗${NC} $name — esperado HTTP $expected, recibido HTTP $actual"
        ((FAIL++))
    fi
}

echo ""
echo -e "${BLUE}╔══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Tool Rental API — Prueba Completa        ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════╝${NC}"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[1] Health Check${NC}"
HEALTH=$(curl -s "$BASE_URL/../health")
check "GET /health" '"status":"ok"' "$HEALTH"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[2] Registro de usuarios${NC}"

TS=$(date +%s)
OWNER_EMAIL="owner_${TS}@test.com"
REQUESTER_EMAIL="req_${TS}@test.com"

# Registrar propietario
REG_OWNER=$(curl -s -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Carlos Propietario\",\"email\":\"$OWNER_EMAIL\",\"password\":\"password123\",\"role\":\"owner\"}")
check "POST /auth/register (owner)" '"role":"owner"' "$REG_OWNER"
OWNER_TOKEN=$(echo "$REG_OWNER" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# Registrar solicitante
REG_REQ=$(curl -s -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Luis Solicitante\",\"email\":\"$REQUESTER_EMAIL\",\"password\":\"password123\",\"role\":\"requester\"}")
check "POST /auth/register (requester)" '"role":"requester"' "$REG_REQ"
REQUESTER_TOKEN=$(echo "$REG_REQ" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

# Email duplicado → 409
DUP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Test User\",\"email\":\"$OWNER_EMAIL\",\"password\":\"password123\",\"role\":\"owner\"}")
check_status "POST /auth/register (email duplicado)" "409" "$DUP_STATUS"

# Body inválido → 400
BAD_BODY=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"name":"X"}')
check_status "POST /auth/register (body inválido)" "400" "$BAD_BODY"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[3] Login${NC}"

LOGIN=$(curl -s -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$OWNER_EMAIL\",\"password\":\"password123\"}")
check "POST /auth/login (credenciales correctas)" '"token"' "$LOGIN"
OWNER_TOKEN=$(echo "$LOGIN" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

LOGIN_BAD=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$OWNER_EMAIL\",\"password\":\"wrongpass\"}")
check_status "POST /auth/login (contraseña incorrecta)" "401" "$LOGIN_BAD"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[4] Precio sugerido${NC}"

PRICING=$(curl -s "$BASE_URL/pricing?estimated_value=3000")
check "GET /pricing?estimated_value=3000" '"suggested_daily_rate"' "$PRICING"
check "GET /pricing — fórmula 50% en 30 días" '"minimum_daily_rate":50' "$PRICING"

BAD_PRICING=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/pricing")
check_status "GET /pricing (sin parámetro)" "400" "$BAD_PRICING"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[5] Gestión de herramientas (Propietario)${NC}"

# Crear herramienta 1
TOOL1=$(curl -s -X POST "$BASE_URL/tools" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{"name":"Taladro Bosch 800W","description":"Percutor para concreto","category":"Construccion","estimated_value":2500,"daily_rate":50}')
check "POST /tools (crear herramienta)" '"name":"Taladro Bosch 800W"' "$TOOL1"
check "POST /tools — precio ajustado al mínimo" '"daily_rate":50' "$TOOL1"
check "POST /tools — precio sugerido en respuesta" '"suggested_min_daily_rate"' "$TOOL1"
TOOL1_ID=$(echo "$TOOL1" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

# Crear herramienta 2
TOOL2=$(curl -s -X POST "$BASE_URL/tools" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{"name":"Lijadora Orbital","description":"Para madera","category":"Carpinteria","estimated_value":1500,"daily_rate":35}')
check "POST /tools (segunda herramienta)" '"name":"Lijadora Orbital"' "$TOOL2"
TOOL2_ID=$(echo "$TOOL2" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

# Sin token → 401
NO_AUTH=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/tools" \
  -H "Content-Type: application/json" \
  -d '{"name":"Hack","estimated_value":100,"daily_rate":5}')
check_status "POST /tools (sin token)" "401" "$NO_AUTH"

# Requester no puede crear herramientas → 403
REQ_TOOL=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/tools" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $REQUESTER_TOKEN" \
  -d '{"name":"Test","estimated_value":100,"daily_rate":5}')
check_status "POST /tools (requester no autorizado)" "403" "$REQ_TOOL"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[6] Catálogo y búsqueda (Solicitante)${NC}"

CATALOG=$(curl -s "$BASE_URL/tools")
check "GET /tools (catálogo público)" '"Taladro Bosch 800W"' "$CATALOG"

SEARCH=$(curl -s "$BASE_URL/tools?search=Taladro")
check "GET /tools?search=Taladro" '"Taladro Bosch 800W"' "$SEARCH"

AVAIL=$(curl -s "$BASE_URL/tools?available=true")
check "GET /tools?available=true" '"is_available":true' "$AVAIL"

DETAIL=$(curl -s "$BASE_URL/tools/$TOOL1_ID")
check "GET /tools/:id (detalle)" '"suggested_min_daily_rate"' "$DETAIL"

MY_TOOLS=$(curl -s -H "Authorization: Bearer $OWNER_TOKEN" "$BASE_URL/owner/tools")
check "GET /owner/tools (mis herramientas)" '"Taladro Bosch 800W"' "$MY_TOOLS"

NOT_FOUND=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/tools/00000000-0000-0000-0000-000000000000")
check_status "GET /tools/:id (no existe)" "404" "$NOT_FOUND"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[7] Actualizar herramienta${NC}"

UPDATE=$(curl -s -X PUT "$BASE_URL/tools/$TOOL1_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{"description":"Actualizado: percutor profesional"}')
check "PUT /tools/:id (actualizar)" '"Actualizado:' "$UPDATE"

FORBIDDEN=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/tools/$TOOL1_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $REQUESTER_TOKEN" \
  -d '{"description":"Hack"}')
check_status "PUT /tools/:id (otro usuario)" "403" "$FORBIDDEN"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[8] Flujo de renta — Crear solicitud${NC}"

START=$(date -u -v+1d +"%Y-%m-%dT10:00:00Z")
END=$(date -u -v+6d +"%Y-%m-%dT10:00:00Z")

RENTAL=$(curl -s -X POST "$BASE_URL/rentals" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $REQUESTER_TOKEN" \
  -d "{\"tool_id\":\"$TOOL1_ID\",\"start_date\":\"$START\",\"end_date\":\"$END\"}")
check "POST /rentals (crear renta)" '"status":"pending"' "$RENTAL"
check "POST /rentals — ambas confirmaciones en false" '"owner_confirmed_delivery":false' "$RENTAL"
check "POST /rentals — total calculado" '"total_amount"' "$RENTAL"
RENTAL_ID=$(echo "$RENTAL" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

# Herramienta ya no disponible
UNAVAIL=$(curl -s "$BASE_URL/tools/$TOOL1_ID")
check "Herramienta → no disponible al rentar" '"is_available":false' "$UNAVAIL"

# Intentar rentar herramienta ocupada → 409
DOUBLE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rentals" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $REQUESTER_TOKEN" \
  -d "{\"tool_id\":\"$TOOL1_ID\",\"start_date\":\"$START\",\"end_date\":\"$END\"}")
check_status "POST /rentals (herramienta ocupada)" "409" "$DOUBLE"

# Fechas inválidas → 400
BAD_DATES=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rentals" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $REQUESTER_TOKEN" \
  -d "{\"tool_id\":\"$TOOL1_ID\",\"start_date\":\"$END\",\"end_date\":\"$START\"}")
check_status "POST /rentals (end < start)" "400" "$BAD_DATES"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[9] Consultar rentas${NC}"

RENTALS_LIST=$(curl -s -H "Authorization: Bearer $REQUESTER_TOKEN" "$BASE_URL/rentals")
check "GET /rentals (lista solicitante)" '"status":"pending"' "$RENTALS_LIST"

OWNER_RENTALS=$(curl -s -H "Authorization: Bearer $OWNER_TOKEN" "$BASE_URL/rentals")
check "GET /rentals (lista propietario)" '"status":"pending"' "$OWNER_RENTALS"

RENTAL_DETAIL=$(curl -s -H "Authorization: Bearer $REQUESTER_TOKEN" "$BASE_URL/rentals/$RENTAL_ID")
check "GET /rentals/:id (detalle)" '"daily_rate"' "$RENTAL_DETAIL"

# Tercero no puede ver renta ajena → 403
OTHER_REG=$(curl -s -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Otro\",\"email\":\"other_${TS}@test.com\",\"password\":\"password123\",\"role\":\"requester\"}")
OTHER_TOKEN=$(echo "$OTHER_REG" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
THIRD_PARTY=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $OTHER_TOKEN" "$BASE_URL/rentals/$RENTAL_ID")
check_status "GET /rentals/:id (tercero)" "403" "$THIRD_PARTY"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[10] Apretón de manos — Entrega${NC}"

# Propietario confirma entrega
CONF_O=$(curl -s -X POST "$BASE_URL/rentals/$RENTAL_ID/confirm-delivery" \
  -H "Authorization: Bearer $OWNER_TOKEN")
check "confirm-delivery (propietario)" '"owner_confirmed_delivery":true' "$CONF_O"
check "Estado sigue pending (falta solicitante)" '"status":"pending"' "$CONF_O"

# Propietario intenta confirmar dos veces → 409
DUP_CONF=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rentals/$RENTAL_ID/confirm-delivery" \
  -H "Authorization: Bearer $OWNER_TOKEN")
check_status "confirm-delivery (propietario duplicado)" "409" "$DUP_CONF"

# Tercero no puede confirmar → 403
THIRD_CONF=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rentals/$RENTAL_ID/confirm-delivery" \
  -H "Authorization: Bearer $OTHER_TOKEN")
check_status "confirm-delivery (tercero)" "403" "$THIRD_CONF"

# Solicitante confirma → pasa a ACTIVE
CONF_R=$(curl -s -X POST "$BASE_URL/rentals/$RENTAL_ID/confirm-delivery" \
  -H "Authorization: Bearer $REQUESTER_TOKEN")
check "confirm-delivery (solicitante) → active" '"status":"active"' "$CONF_R"
check "Ambas confirmaciones verdaderas" '"requester_confirmed_delivery":true' "$CONF_R"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[11] Apretón de manos — Devolución${NC}"

# No se puede confirmar entrega si ya es active → 409
LATE_DELIVERY=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rentals/$RENTAL_ID/confirm-delivery" \
  -H "Authorization: Bearer $REQUESTER_TOKEN")
check_status "confirm-delivery en renta active → 409" "409" "$LATE_DELIVERY"

# Solicitante confirma devolución
RET_R=$(curl -s -X POST "$BASE_URL/rentals/$RENTAL_ID/confirm-return" \
  -H "Authorization: Bearer $REQUESTER_TOKEN")
check "confirm-return (solicitante)" '"requester_confirmed_return":true' "$RET_R"
check "Estado sigue active (falta propietario)" '"status":"active"' "$RET_R"

# Solicitante intenta confirmar dos veces → 409
DUP_RET=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rentals/$RENTAL_ID/confirm-return" \
  -H "Authorization: Bearer $REQUESTER_TOKEN")
check_status "confirm-return (solicitante duplicado)" "409" "$DUP_RET"

# Propietario confirma → COMPLETED (fondos liberados)
RET_O=$(curl -s -X POST "$BASE_URL/rentals/$RENTAL_ID/confirm-return" \
  -H "Authorization: Bearer $OWNER_TOKEN")
check "confirm-return (propietario) → completed" '"status":"completed"' "$RET_O"
check "Ambas devoluciones confirmadas" '"owner_confirmed_return":true' "$RET_O"

# Herramienta vuelve a estar disponible
AVAIL_BACK=$(curl -s "$BASE_URL/tools/$TOOL1_ID")
check "Herramienta disponible de nuevo" '"is_available":true' "$AVAIL_BACK"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[12] Cancelar renta${NC}"

RENTAL2=$(curl -s -X POST "$BASE_URL/rentals" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $REQUESTER_TOKEN" \
  -d "{\"tool_id\":\"$TOOL1_ID\",\"start_date\":\"$START\",\"end_date\":\"$END\"}")
RENTAL2_ID=$(echo "$RENTAL2" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

CANCEL=$(curl -s -X DELETE "$BASE_URL/rentals/$RENTAL2_ID" \
  -H "Authorization: Bearer $REQUESTER_TOKEN")
check "DELETE /rentals/:id (cancelar)" '"status":"cancelled"' "$CANCEL"

AVAIL_CANCEL=$(curl -s "$BASE_URL/tools/$TOOL1_ID")
check "Herramienta disponible tras cancelar" '"is_available":true' "$AVAIL_CANCEL"

# No se puede cancelar una renta ya completada → 409
CANCEL_DONE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/rentals/$RENTAL_ID" \
  -H "Authorization: Bearer $OWNER_TOKEN")
check_status "DELETE /rentals/:id (ya completada)" "409" "$CANCEL_DONE"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[13] Mercado Pago mock + contrato SHA-256 + disputa${NC}"

# Crear renta con card_token (mock: acepta cualquier string)
RENTAL_PAY=$(curl -s -X POST "$BASE_URL/rentals" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $REQUESTER_TOKEN" \
  -d "{\"tool_id\":\"$TOOL1_ID\",\"start_date\":\"$(date -u -v+3d +%Y-%m-%dT10:00:00Z)\",\"end_date\":\"$(date -u -v+6d +%Y-%m-%dT10:00:00Z)\",\"card_token\":\"TEST-card-mock\",\"payer_email\":\"solicitante@test.com\"}")
PAY_RENTAL_ID=$(echo "$RENTAL_PAY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
check "Crear renta con card_token (pago mock)" '"mp_payment_id":"MOCK_' "$RENTAL_PAY"
check "deductible_amount > 0" '"deductible_amount":' "$RENTAL_PAY"

# Confirmar entrega CON GPS (propietario)
DEL_GPS_OWNER=$(curl -s -X POST "$BASE_URL/rentals/$PAY_RENTAL_ID/confirm-delivery" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{"latitude":19.432608,"longitude":-99.133209}')
check "confirm-delivery (propietario) con GPS" '"owner_confirmed_delivery":true' "$DEL_GPS_OWNER"

# Confirmar entrega (solicitante) → debe generar contrato SHA-256
DEL_GPS_REQ=$(curl -s -X POST "$BASE_URL/rentals/$PAY_RENTAL_ID/confirm-delivery" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $REQUESTER_TOKEN" \
  -d '{"latitude":19.432608,"longitude":-99.133209}')
check "confirm-delivery (solicitante) → active" '"status":"active"' "$DEL_GPS_REQ"
check "contrato SHA-256 generado" '"contract_hash":"' "$DEL_GPS_REQ"
check "GPS registrado en delivery_lat" '"delivery_lat":19' "$DEL_GPS_REQ"
check "delivery_at registrado" '"delivery_at":"' "$DEL_GPS_REQ"

# Propietario reporta disputa
DISPUTE_RESP=$(curl -s -X POST "$BASE_URL/rentals/$PAY_RENTAL_ID/dispute" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{"reason":"La sierra llegó con la hoja partida y el motor trabado"}')
check "dispute → status disputed" '"status":"disputed"' "$DISPUTE_RESP"
check "dispute_reason guardado" '"dispute_reason":' "$DISPUTE_RESP"

# Solicitante NO puede disputar (403)
DISPUTE_UNAUTH=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rentals/$PAY_RENTAL_ID/dispute" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $REQUESTER_TOKEN" \
  -d '{"reason":"intento de fraude"}')
check_status "dispute por solicitante (HTTP 403)" "403" "$DISPUTE_UNAUTH"

# No se puede disputar una renta ya disputada (409 — no está active)
DISPUTE_AGAIN=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/rentals/$PAY_RENTAL_ID/dispute" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{"reason":"segundo intento"}')
check_status "dispute doble (HTTP 409)" "409" "$DISPUTE_AGAIN"

# Webhook de Mercado Pago (ruta pública)
WH=$(curl -s -X POST "$BASE_URL/webhooks/mercadopago" \
  -H "Content-Type: application/json" \
  -d '{"id":"12345","type":"payment","action":"payment.updated","data":{"id":"MOCK_999"}}')
check "webhook MP (received: true)" '"received":true' "$WH"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo -e "${CYAN}[14] Eliminar herramienta${NC}"

TOOL3=$(curl -s -X POST "$BASE_URL/tools" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d '{"name":"Herramienta temporal","estimated_value":500,"daily_rate":20}')
TOOL3_ID=$(echo "$TOOL3" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

DEL=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/tools/$TOOL3_ID" \
  -H "Authorization: Bearer $OWNER_TOKEN")
check_status "DELETE /tools/:id" "200" "$DEL"

DELETED=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/tools/$TOOL3_ID")
check_status "GET /tools/:id (eliminada → 404)" "404" "$DELETED"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
TOTAL=$((PASS + FAIL))
PCT=$(awk "BEGIN {printf \"%.0f\", $PASS * 100 / $TOTAL}")

echo -e "${BLUE}╔══════════════════════════════════════════════╗${NC}"
printf "${BLUE}║${NC}  Resultados: ${GREEN}%-3s ✓${NC}  ${RED}%-3s ✗${NC}  de ${TOTAL} pruebas        ${BLUE}║${NC}\n" "$PASS" "$FAIL"
printf "${BLUE}║${NC}  Porcentaje de éxito: "
if [ "$PCT" -ge 95 ]; then
    printf "${GREEN}%s%%${NC}" "$PCT"
else
    printf "${RED}%s%%${NC}" "$PCT"
fi
printf "                          ${BLUE}║${NC}\n"
echo -e "${BLUE}╚══════════════════════════════════════════════╝${NC}"
echo ""

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
