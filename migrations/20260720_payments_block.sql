-- =============================================================================
-- Migración: Bloque de pagos — comisión de servicio, tarjetas guardadas y
-- seguro de herramientas
-- ToolShare · PostgreSQL
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1) RENTALS: comisión de servicio de ToolShare (cargo adicional, no reembolsable)
-- -----------------------------------------------------------------------------
-- Distinta del deductible_amount (depósito de garantía, reembolsable salvo
-- disputa): la comisión es ingreso de la plataforma, se cobra siempre junto
-- con el total de la renta.
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS commission_amount NUMERIC(12,2) NOT NULL DEFAULT 0;

-- -----------------------------------------------------------------------------
-- 2) USERS: referencia al Customer de Mercado Pago (para tarjetas guardadas)
-- -----------------------------------------------------------------------------
ALTER TABLE users ADD COLUMN IF NOT EXISTS mp_customer_id VARCHAR(64);

-- -----------------------------------------------------------------------------
-- 3) SAVED_CARDS: tarjetas guardadas por usuario vía Mercado Pago Customer/Card API
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS saved_cards (
    id                UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID          NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mp_card_id        VARCHAR(64)   NOT NULL,
    card_brand        VARCHAR(30)   NOT NULL DEFAULT '',
    last_four_digits  VARCHAR(4)    NOT NULL DEFAULT '',
    expiration_month  SMALLINT,
    expiration_year   SMALLINT,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT saved_cards_unique_mp_card UNIQUE (user_id, mp_card_id)
);

CREATE INDEX IF NOT EXISTS idx_saved_cards_user_id ON saved_cards (user_id);

-- -----------------------------------------------------------------------------
-- 4) TOOLS: intención de seguro (Respaldo ToolShare)
-- -----------------------------------------------------------------------------
ALTER TABLE tools ADD COLUMN IF NOT EXISTS wants_insurance BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE tools ADD COLUMN IF NOT EXISTS insurance_monthly_premium NUMERIC(12,2) NOT NULL DEFAULT 0;
