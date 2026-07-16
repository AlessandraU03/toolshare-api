-- =============================================================================
-- Migración: Reseñas y calificación por estrellas (1-5)
-- ToolShare · PostgreSQL
-- =============================================================================
-- Un propietario puede calificar al solicitante (reputación como arrendatario)
-- y un solicitante puede calificar la herramienta al terminar la renta.
-- Solo se permite una reseña por dirección (rental_id, author_id) y únicamente
-- cuando la renta ya está en estado 'completed'.
-- =============================================================================

CREATE TABLE IF NOT EXISTS reviews (
    id          UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    rental_id   UUID          NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
    author_id   UUID          NOT NULL REFERENCES users(id),
    target_type VARCHAR(10)   NOT NULL CHECK (target_type IN ('tool', 'user')),
    target_id   UUID          NOT NULL,
    rating      SMALLINT      NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment     TEXT          NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT reviews_one_per_direction UNIQUE (rental_id, author_id)
);

CREATE INDEX IF NOT EXISTS idx_reviews_target ON reviews(target_type, target_id);
