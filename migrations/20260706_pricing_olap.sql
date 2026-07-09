-- =============================================================================
-- Migración: Soporte para Motor de Precios (CNN + K-Means + Isolation Forest +
-- Random Forest) y capa analítica OLAP (DuckDB)
-- ToolShare · PostgreSQL
-- =============================================================================
-- Este script asume que el esquema base (users, tools, rentals, rental_messages)
-- ya existe. Solo agrega lo necesario para el pipeline de pricing y analítica.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- 1) TOOLS: condición física (CNN), marca, antigüedad y ubicación jerárquica
-- -----------------------------------------------------------------------------
ALTER TABLE tools ADD COLUMN IF NOT EXISTS condition_score FLOAT;
    -- Score continuo (0.0 - 1.0) generado por el modelo CNN.
    -- 0 = muy desgastada, 1 = como nueva.

ALTER TABLE tools ADD COLUMN IF NOT EXISTS brand VARCHAR(100);
    -- Marca declarada por el propietario (feature de pricing).

ALTER TABLE tools ADD COLUMN IF NOT EXISTS age_months INT;
    -- Antigüedad aproximada en meses (usada en la fórmula de depreciación).

ALTER TABLE tools ADD COLUMN IF NOT EXISTS city VARCHAR(100);
ALTER TABLE tools ADD COLUMN IF NOT EXISTS state VARCHAR(100);
    -- Jerarquía geográfica (city < state) para la dimensión de zona en el cubo OLAP.
    -- Se puede derivar por geocodificación inversa de latitude/longitude.

ALTER TABLE tools ADD COLUMN IF NOT EXISTS price_source VARCHAR(30)
    DEFAULT 'catalogo_semilla'
    CHECK (price_source IN ('ticket_validado', 'modelo_depreciacion', 'catalogo_semilla'));
    -- Trazabilidad de dónde salió el estimated_value:
    --  - ticket_validado: el usuario subió comprobante de compra (OCR)
    --  - modelo_depreciacion: se calculó automáticamente (precio_nuevo x depreciación x condición)
    --  - catalogo_semilla: aún no hay suficiente info, se usa el valor base por categoría

CREATE INDEX IF NOT EXISTS idx_tools_brand    ON tools (brand);
CREATE INDEX IF NOT EXISTS idx_tools_city     ON tools (city);

-- -----------------------------------------------------------------------------
-- 2) RENTALS: flag de outlier para el Isolation Forest
-- -----------------------------------------------------------------------------
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS is_outlier BOOLEAN NOT NULL DEFAULT FALSE;
    -- Se actualiza por el job periódico de Isolation Forest.
    -- Las filas marcadas TRUE se excluyen del reentrenamiento del Random Forest.

CREATE INDEX IF NOT EXISTS idx_rentals_is_outlier ON rentals (is_outlier);
CREATE INDEX IF NOT EXISTS idx_rentals_status_outlier ON rentals (status, is_outlier);
    -- Acelera la consulta típica: WHERE status = 'completed' AND is_outlier = false

-- -----------------------------------------------------------------------------
-- 3) CATÁLOGO SEMILLA: precios de referencia para el arranque en frío (K-Means)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS catalogo_semilla (
    id            SERIAL PRIMARY KEY,
    category      VARCHAR(100) NOT NULL,
    precio_base   NUMERIC(12,2) NOT NULL,   -- precio de renta diario de referencia
    valor_nuevo   NUMERIC(12,2) NOT NULL,   -- precio aproximado del producto nuevo
    cluster_id    INT,                       -- asignado por K-Means
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_catalogo_semilla_category ON catalogo_semilla (category);

-- -----------------------------------------------------------------------------
-- 4) MODELOS ENTRENADOS: bitácora de versiones (para trazabilidad académica)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pricing_model_runs (
    id                SERIAL PRIMARY KEY,
    model_type        VARCHAR(30) NOT NULL
                       CHECK (model_type IN ('kmeans_semilla', 'isolation_forest', 'random_forest')),
    n_registros       INT,               -- cuántas filas se usaron para entrenar
    metric_name       VARCHAR(30),       -- ej. 'r2_score', 'contamination'
    metric_value      FLOAT,
    trained_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notes             TEXT
);

-- -----------------------------------------------------------------------------
-- 5) FACTOR DE AJUSTE TEMPORAL (media móvil mensual, reemplaza a INEGI/terceros)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS factor_ajuste_mensual (
    id              SERIAL PRIMARY KEY,
    category        VARCHAR(100) NOT NULL,
    anio            INT NOT NULL,
    mes             INT NOT NULL,
    precio_promedio NUMERIC(12,2) NOT NULL,
    factor_ajuste   FLOAT,             -- precio_promedio_mes_actual / precio_promedio_mes_anterior
    calculado_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (category, anio, mes)
);

-- -----------------------------------------------------------------------------
DROP VIEW IF EXISTS v_conteo_transacciones_categoria CASCADE;
DROP VIEW IF EXISTS v_transacciones_limpias CASCADE;
CREATE VIEW v_transacciones_limpias AS
SELECT
    r.id              AS rental_id,
    t.id              AS tool_id,
    r.requester_id,
    t.category,
    t.brand,
    t.age_months,
    t.condition_score,
    t.estimated_value,
    t.city,
    t.state,
    r.daily_rate,
    r.total_amount,
    r.start_date,
    r.end_date
FROM rentals r
JOIN tools t ON r.tool_id = t.id
WHERE r.status = 'completed'
  AND r.is_outlier = false;

-- -----------------------------------------------------------------------------
-- 7) VISTA: Conteo de transacciones por categoría (para decidir cold-start vs RF)
-- -----------------------------------------------------------------------------
CREATE OR REPLACE VIEW v_conteo_transacciones_categoria AS
SELECT category, COUNT(*) AS n_transacciones
FROM v_transacciones_limpias
GROUP BY category;
