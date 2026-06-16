-- =============================================================================
-- Migración 003: Pago con Mercado Pago (pre-autorización) + contrato SHA-256 + GPS
-- =============================================================================

-- Ampliar CHECK para incluir el estado de disputa
ALTER TABLE rentals DROP CONSTRAINT IF EXISTS rentals_status_check;
ALTER TABLE rentals
    ADD CONSTRAINT rentals_status_check
    CHECK (status IN ('pending','active','completed','cancelled','disputed'));

-- Campos de pago Mercado Pago
ALTER TABLE rentals
    ADD COLUMN IF NOT EXISTS mp_payment_id     TEXT,
    ADD COLUMN IF NOT EXISTS payment_status    TEXT,
    ADD COLUMN IF NOT EXISTS deductible_amount NUMERIC(12,2) NOT NULL DEFAULT 0;

-- Contrato digital (SHA-256) + GPS en la entrega
ALTER TABLE rentals
    ADD COLUMN IF NOT EXISTS contract_hash TEXT,
    ADD COLUMN IF NOT EXISTS delivery_lat  DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS delivery_lng  DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS delivery_at   TIMESTAMPTZ;

-- Motivo de disputa (propietario rechaza devolución)
ALTER TABLE rentals
    ADD COLUMN IF NOT EXISTS dispute_reason TEXT;
