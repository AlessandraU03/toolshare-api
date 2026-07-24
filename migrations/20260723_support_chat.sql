-- =============================================================================
-- Migración: Chat de soporte entre el Administrador y el Propietario
-- ToolShare · PostgreSQL
-- =============================================================================
-- Canal general por propietario (no ligado a ninguna renta) para dudas o
-- aclaraciones sobre el uso de la plataforma. Cualquier admin puede leer y
-- responder el hilo de un propietario.

CREATE TABLE IF NOT EXISTS support_messages (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sender_id  UUID        NOT NULL REFERENCES users(id),
    message    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_messages_owner_id ON support_messages (owner_id);
