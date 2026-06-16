-- =============================================================================
-- Migración 002: Campos MVP ToolShare
-- Ejecutar SOLO una vez sobre una BD que ya tiene la migración 001 aplicada.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- TABLA users: agregar phone e ine_number
-- -----------------------------------------------------------------------------
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS phone      VARCHAR(20),
    ADD COLUMN IF NOT EXISTS ine_number VARCHAR(50);

-- -----------------------------------------------------------------------------
-- TABLA tools: agregar campos de detalle, ubicación y precio
-- -----------------------------------------------------------------------------
ALTER TABLE tools
    ADD COLUMN IF NOT EXISTS brand         VARCHAR(100),
    ADD COLUMN IF NOT EXISTS model         VARCHAR(100),
    ADD COLUMN IF NOT EXISTS wear_level    VARCHAR(50)
                                           CHECK (wear_level IN ('Nuevo', 'Buen Estado', 'Desgastado')),
    ADD COLUMN IF NOT EXISTS latitude      DECIMAL(10, 7),
    ADD COLUMN IF NOT EXISTS longitude     DECIMAL(10, 7),
    ADD COLUMN IF NOT EXISTS price_per_day DECIMAL(10, 2),
    ADD COLUMN IF NOT EXISTS photo_url     VARCHAR(500);

-- Índice de búsqueda por categoría + disponibilidad (usado en catálogo)
CREATE INDEX IF NOT EXISTS idx_tools_price ON tools (price_per_day);

-- -----------------------------------------------------------------------------
-- TABLA rentals: órdenes de renta (apretón de manos digital)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS rentals (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_id         UUID         NOT NULL REFERENCES tools(id) ON DELETE RESTRICT,
    requester_id    UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    -- Snapshot de precios al momento de crear la orden (inmutable)
    days            INTEGER      NOT NULL CHECK (days > 0),
    price_per_day   DECIMAL(10,2) NOT NULL CHECK (price_per_day >= 0),
    total           DECIMAL(10,2) NOT NULL CHECK (total >= 0),
    deposit         DECIMAL(10,2) NOT NULL CHECK (deposit >= 0),
    -- Estado del flujo de renta
    status          VARCHAR(30)  NOT NULL DEFAULT 'pending_payment'
                                 CHECK (status IN (
                                     'pending_payment',   -- esperando pago
                                     'funds_held',        -- fondos retenidos
                                     'delivered',         -- entregada físicamente
                                     'in_use',            -- en uso por solicitante
                                     'returned',          -- devuelta, esperando confirmación del dueño
                                     'completed',         -- finalizada correctamente
                                     'dispute',           -- en disputa / arbitraje
                                     'cancelled'          -- cancelada
                                 )),
    -- Datos de Mercado Pago
    payment_id      VARCHAR(100),          -- ID de pago devuelto por MP
    payment_url     VARCHAR(500),          -- URL del checkout de MP
    -- Confirmación de entrega (GPS)
    requester_confirmed_delivery BOOLEAN  DEFAULT FALSE,
    owner_confirmed_delivery     BOOLEAN  DEFAULT FALSE,
    delivery_latitude   DECIMAL(10, 7),
    delivery_longitude  DECIMAL(10, 7),
    -- Hash del "contrato digital" generado al confirmar entrega
    contract_hash   VARCHAR(100),
    -- Timestamps
    expires_at          TIMESTAMPTZ,
    delivery_confirmed_at TIMESTAMPTZ,
    return_requested_at   TIMESTAMPTZ,
    return_accepted       BOOLEAN,
    return_rejection_reason TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Índices para consultas del panel de usuario
CREATE INDEX IF NOT EXISTS idx_rentals_tool_id      ON rentals (tool_id);
CREATE INDEX IF NOT EXISTS idx_rentals_requester_id ON rentals (requester_id);
CREATE INDEX IF NOT EXISTS idx_rentals_status       ON rentals (status);

-- Trigger para updated_at en rentals
DROP TRIGGER IF EXISTS trg_rentals_updated_at ON rentals;
CREATE TRIGGER trg_rentals_updated_at
    BEFORE UPDATE ON rentals
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
