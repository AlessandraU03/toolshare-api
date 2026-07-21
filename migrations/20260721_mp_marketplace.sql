-- =============================================================================
-- Migración: Vínculo de cuenta de Mercado Pago del propietario (Marketplace/OAuth)
-- ToolShare · PostgreSQL
-- =============================================================================
-- Permite que cada propietario conecte su propia cuenta de Mercado Pago para
-- que los pagos se dividan automáticamente en cada transacción: la comisión
-- de servicio queda en la cuenta de la plataforma y el resto se deposita
-- directo en la cuenta del propietario (application_fee / marketplace_fee).

ALTER TABLE users ADD COLUMN IF NOT EXISTS mp_seller_user_id VARCHAR(64);
ALTER TABLE users ADD COLUMN IF NOT EXISTS mp_seller_access_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS mp_seller_refresh_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS mp_seller_token_expires_at TIMESTAMPTZ;
