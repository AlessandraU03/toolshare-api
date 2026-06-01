-- =============================================================================
-- Migración 001: Esquema inicial para la API de Inventario de Herramientas
-- =============================================================================

-- Extensión para UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================================================
-- TABLA: users
-- Almacena tanto Propietarios como Solicitantes.
-- El campo 'role' distingue los permisos de cada usuario.
-- =============================================================================
CREATE TABLE IF NOT EXISTS users (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       VARCHAR(100) NOT NULL,
    email      VARCHAR(255) NOT NULL UNIQUE,
    password   VARCHAR(255) NOT NULL,  -- Hash bcrypt
    role       VARCHAR(20)  NOT NULL DEFAULT 'requester'
                            CHECK (role IN ('owner', 'requester')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índice en email para búsquedas de login rápidas
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

-- =============================================================================
-- TABLA: tools
-- Catálogo de herramientas. Cada herramienta pertenece a un Propietario.
-- =============================================================================
CREATE TABLE IF NOT EXISTS tools (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id    UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        VARCHAR(150) NOT NULL,
    description TEXT,
    category    VARCHAR(100),
    is_available BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Índices para consultas frecuentes
CREATE INDEX IF NOT EXISTS idx_tools_owner_id      ON tools (owner_id);
CREATE INDEX IF NOT EXISTS idx_tools_is_available  ON tools (is_available);
CREATE INDEX IF NOT EXISTS idx_tools_category      ON tools (category);

-- =============================================================================
-- FUNCIÓN + TRIGGER: Actualiza 'updated_at' automáticamente en cada UPDATE
-- =============================================================================
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_tools_updated_at
    BEFORE UPDATE ON tools
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =============================================================================
-- DATOS DE PRUEBA (seed)
-- Contraseña para ambos usuarios: "password123"
-- Hash bcrypt generado con cost=10
-- =============================================================================