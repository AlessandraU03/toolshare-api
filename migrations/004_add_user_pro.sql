-- =============================================================================
-- Migración 004: Añadir columna is_pro para el plan premium de usuarios
-- =============================================================================

ALTER TABLE users 
    ADD COLUMN IF NOT EXISTS is_pro BOOLEAN NOT NULL DEFAULT FALSE;
