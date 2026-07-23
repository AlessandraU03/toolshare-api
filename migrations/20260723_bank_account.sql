-- =============================================================================
-- Migración: Datos bancarios del propietario para pagos externos por disputa
-- ToolShare · PostgreSQL
-- =============================================================================
-- Cuando el administrador resuelve una disputa a favor del propietario y la
-- herramienta tiene el seguro ToolShare activo, la plataforma le debe un pago
-- adicional (30% del valor estimado) que no se puede transferir automático
-- vía Mercado Pago (no hay API de "enviar dinero" disponible con nuestra
-- integración actual). Se guardan estos datos para que el administrador haga
-- la transferencia manual por fuera de la app.

ALTER TABLE users ADD COLUMN IF NOT EXISTS bank_clabe VARCHAR(18);
ALTER TABLE users ADD COLUMN IF NOT EXISTS bank_account_holder VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS bank_name VARCHAR(100);
