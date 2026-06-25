-- =============================================================================
-- Esquema de Base de Datos Unificado para ToolShare
-- PostgresSQL
-- =============================================================================

-- Habilitar extensión para generación de UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Limpiar tablas existentes para actualizar el esquema local
DROP TABLE IF EXISTS rentals, tools, users CASCADE;

-- =============================================================================
-- TABLA: users
-- Almacena tanto Propietarios (owner) como Solicitantes (requester).
-- =============================================================================
CREATE TABLE IF NOT EXISTS users (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       VARCHAR(100) NOT NULL,
    email      VARCHAR(255) NOT NULL UNIQUE,
    password   VARCHAR(255) NOT NULL,  -- Hash bcrypt
    role       VARCHAR(20)  NOT NULL DEFAULT 'requester'
                            CHECK (role IN ('owner', 'requester')),
    is_pro     BOOLEAN      NOT NULL DEFAULT FALSE,
    phone      VARCHAR(20)  NOT NULL DEFAULT '',
    ine        VARCHAR(50)  NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índice para optimizar búsquedas de login
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

-- =============================================================================
-- TABLA: tools
-- Catálogo de herramientas de construcción. Cada herramienta pertenece a un owner.
-- =============================================================================
CREATE TABLE IF NOT EXISTS tools (
    id              UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id        UUID           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(150)   NOT NULL,
    description     TEXT,
    category        VARCHAR(100),
    photo_url       TEXT,
    estimated_value NUMERIC(12,2)  NOT NULL DEFAULT 0,
    daily_rate      NUMERIC(12,2)  NOT NULL DEFAULT 0,
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    is_available    BOOLEAN        NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Índices para búsquedas de herramientas
CREATE INDEX IF NOT EXISTS idx_tools_owner_id      ON tools (owner_id);
CREATE INDEX IF NOT EXISTS idx_tools_is_available  ON tools (is_available);
CREATE INDEX IF NOT EXISTS idx_tools_category      ON tools (category);

-- =============================================================================
-- TABLA: rentals
-- Registra cada contrato de alquiler y depósitos en garantía.
-- =============================================================================
CREATE TABLE IF NOT EXISTS rentals (
    id                           UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_id                      UUID          NOT NULL REFERENCES tools(id),
    requester_id                 UUID          NOT NULL REFERENCES users(id),
    owner_id                     UUID          NOT NULL REFERENCES users(id),
    start_date                   TIMESTAMPTZ   NOT NULL,
    end_date                     TIMESTAMPTZ   NOT NULL,
    daily_rate                   NUMERIC(12,2) NOT NULL,
    total_amount                 NUMERIC(12,2) NOT NULL,
    status                       VARCHAR(20)   NOT NULL DEFAULT 'pending'
                                 CHECK (status IN ('pending','active','completed','cancelled','disputed')),
    
    -- Apretón de manos digital (entrega y devolución)
    owner_confirmed_delivery     BOOLEAN       NOT NULL DEFAULT FALSE,
    requester_confirmed_delivery BOOLEAN       NOT NULL DEFAULT FALSE,
    requester_confirmed_return   BOOLEAN       NOT NULL DEFAULT FALSE,
    owner_confirmed_return       BOOLEAN       NOT NULL DEFAULT FALSE,

    -- Pasarela de Pagos (Mercado Pago)
    mp_payment_id                TEXT,
    payment_status               TEXT,
    deductible_amount            NUMERIC(12,2) NOT NULL DEFAULT 0,

    -- Contrato digital firmado con hash SHA-256 y ubicación GPS en la entrega
    contract_hash                TEXT,
    delivery_lat                 DOUBLE PRECISION,
    delivery_lng                 DOUBLE PRECISION,
    delivery_at                  TIMESTAMPTZ,

    -- Disputas (Rechazo de devolución)
    dispute_reason               TEXT,

    created_at                   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at                   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT rentals_dates_check CHECK (end_date > start_date)
);

-- Índices de alquileres
CREATE INDEX IF NOT EXISTS idx_rentals_tool_id      ON rentals (tool_id);
CREATE INDEX IF NOT EXISTS idx_rentals_requester_id ON rentals (requester_id);
CREATE INDEX IF NOT EXISTS idx_rentals_owner_id     ON rentals (owner_id);
CREATE INDEX IF NOT EXISTS idx_rentals_status       ON rentals (status);

-- =============================================================================
-- PROCEDIMIENTO Y TRIGGERS: Actualización automática de updated_at
-- =============================================================================
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger para users
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Trigger para tools
DROP TRIGGER IF EXISTS trg_tools_updated_at ON tools;
CREATE TRIGGER trg_tools_updated_at
    BEFORE UPDATE ON tools
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Trigger para rentals
DROP TRIGGER IF EXISTS trg_rentals_updated_at ON rentals;
CREATE TRIGGER trg_rentals_updated_at
    BEFORE UPDATE ON rentals
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
