-- =============================================================================
-- Migración: Múltiples fotos por herramienta con score de condición por foto
-- ToolShare · PostgreSQL
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1) TOOL_PHOTOS: una fila por foto subida, cada una con su propio score de
--    la CNN. tools.condition_score pasa a ser el MINIMO (peor caso) de todas
--    las fotos de esa herramienta — una sola foto que muestre desgaste real
--    basta para que se refleje en el precio, en vez de diluirse en un
--    promedio con fotos más favorecedoras.
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tool_photos (
    id              SERIAL         PRIMARY KEY,
    tool_id         UUID           NOT NULL REFERENCES tools(id) ON DELETE CASCADE,
    photo_url       TEXT           NOT NULL,
    condition_score FLOAT          NOT NULL,
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tool_photos_tool_id ON tool_photos(tool_id);

-- -----------------------------------------------------------------------------
-- 2) Nota sobre tools.photo_url / tools.condition_score (sin cambios de
--    esquema aquí): se mantienen como columnas — se siguen leyendo tal cual
--    en pricing_engine.py, las vistas OLAP y el resto del código existente —
--    pero ahora las actualiza la app automáticamente como resumen de
--    tool_photos (photo_url = última foto subida, condition_score = MIN)
--    en vez de recibir el valor directo del cliente.
-- -----------------------------------------------------------------------------
