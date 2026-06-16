-- =============================================================================
-- Migración 002: Precios en herramientas + tabla de rentas con aceptación mutua
-- =============================================================================

-- Agregar campos de precio a la tabla tools
ALTER TABLE tools
    ADD COLUMN IF NOT EXISTS photo_url       TEXT,
    ADD COLUMN IF NOT EXISTS estimated_value NUMERIC(12,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS daily_rate      NUMERIC(12,2) NOT NULL DEFAULT 0;

-- =============================================================================
-- TABLA: rentals
-- Registra cada renta de herramienta entre un propietario y un solicitante.
-- El campo 'status' sigue el ciclo: pending → active → completed | cancelled
--
-- Aceptación mutua (apretón de manos digital):
--   Entrega: owner_confirmed_delivery + requester_confirmed_delivery → active
--   Devolución: requester_confirmed_return + owner_confirmed_return  → completed
-- =============================================================================
CREATE TABLE IF NOT EXISTS rentals (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_id      UUID         NOT NULL REFERENCES tools(id),
    requester_id UUID         NOT NULL REFERENCES users(id),
    owner_id     UUID         NOT NULL REFERENCES users(id),
    start_date   TIMESTAMPTZ  NOT NULL,
    end_date     TIMESTAMPTZ  NOT NULL,
    daily_rate   NUMERIC(12,2) NOT NULL,
    total_amount NUMERIC(12,2) NOT NULL,
    status       VARCHAR(20)  NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending','active','completed','cancelled')),

    -- Apretón de manos: entrega
    owner_confirmed_delivery     BOOLEAN NOT NULL DEFAULT FALSE,
    requester_confirmed_delivery BOOLEAN NOT NULL DEFAULT FALSE,

    -- Apretón de manos: devolución (libera fondos)
    requester_confirmed_return BOOLEAN NOT NULL DEFAULT FALSE,
    owner_confirmed_return     BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT rentals_dates_check CHECK (end_date > start_date)
);

CREATE INDEX IF NOT EXISTS idx_rentals_tool_id      ON rentals (tool_id);
CREATE INDEX IF NOT EXISTS idx_rentals_requester_id ON rentals (requester_id);
CREATE INDEX IF NOT EXISTS idx_rentals_owner_id     ON rentals (owner_id);
CREATE INDEX IF NOT EXISTS idx_rentals_status       ON rentals (status);

CREATE TRIGGER trg_rentals_updated_at
    BEFORE UPDATE ON rentals
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
