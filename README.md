# Tool Inventory API 🔧

API RESTful en Go para gestión de inventario y préstamo de herramientas.
Construida con **Gin**, **pgx** y **JWT**.

---

## Estructura del Proyecto

```
tool-inventory-api/
├── cmd/
│   └── server/
│       └── main.go              # Punto de entrada: router, servidor HTTP
├── internal/
│   ├── auth/
│   │   └── jwt.go               # Generación y validación de tokens JWT
│   ├── database/
│   │   └── database.go          # Conexión y pool pgxpool
│   ├── handler/
│   │   ├── auth_handler.go      # POST /register, POST /login
│   │   └── tool_handler.go      # CRUD de herramientas
│   ├── middleware/
│   │   └── auth.go              # RequireAuth y RequireRole
│   ├── model/
│   │   └── model.go             # Structs: User, Tool, DTOs
│   └── repository/
│       ├── user_repository.go   # Queries SQL de usuarios
│       └── tool_repository.go   # Queries SQL de herramientas
├── migrations/
│   └── 001_initial_schema.sql   # Tablas, índices y seed data
├── scripts/
│   └── setup.sh                 # Script de configuración inicial
├── .env.example                 # Plantilla de variables de entorno
├── go.mod
└── README.md
```

---

## Instalación paso a paso (Terminal integrada de VS Code)

### Requisitos previos
- Go 1.22 o superior: https://go.dev/dl/
- PostgreSQL 14 o superior corriendo localmente

### 1. Crear la base de datos en PostgreSQL

Abre una terminal y ejecuta:

```bash
psql -U postgres -c "CREATE DATABASE tool_inventory;"
```

### 2. Clonar / abrir el proyecto en VS Code

Si empiezas desde cero con este código:
```bash
# Crea la carpeta y entra en ella
mkdir tool-inventory-api && cd tool-inventory-api
# Copia todos los archivos del proyecto aquí
code .   # Abre VS Code en esta carpeta
```

### 3. Abrir la terminal integrada de VS Code

`Ctrl + `` ` (acento grave) o menú **Terminal → New Terminal**

### 4. Inicializar el módulo Go

```bash
# Inicializa el módulo (ya incluido en go.mod, pero si lo haces desde cero):
go mod init github.com/yourusername/tool-inventory-api

# Instala todas las dependencias declaradas en go.mod:
go mod tidy
```

Dependencias que se instalarán automáticamente:
- `github.com/gin-gonic/gin` — Router HTTP
- `github.com/golang-jwt/jwt/v5` — Tokens JWT
- `github.com/google/uuid` — UUIDs v4
- `github.com/jackc/pgx/v5` — Driver PostgreSQL de alto rendimiento
- `github.com/joho/godotenv` — Cargar variables desde .env
- `golang.org/x/crypto` — bcrypt para contraseñas

### 5. Configurar las variables de entorno

```bash
# Copiar la plantilla
cp .env.example .env
```

Edita `.env` con tus datos reales:
```env
SERVER_PORT=8080
DATABASE_URL=postgres://postgres:TU_CONTRASEÑA@localhost:5432/tool_inventory?sslmode=disable
JWT_SECRET=una_clave_secreta_muy_larga_minimo_32_caracteres
JWT_EXPIRATION=24h
```

### 6. Aplicar la migración SQL

```bash
psql -U postgres -d tool_inventory -f migrations/001_initial_schema.sql
```

> El script crea las tablas `users` y `tools`, los índices, triggers
> y dos usuarios de prueba con contraseña `password123`.

### 7. Levantar el servidor

```bash
go run ./cmd/server/main.go
```

Deberías ver:
```
✅ Conexión a PostgreSQL establecida correctamente
🚀 Servidor escuchando en http://localhost:8080
```

---

## Endpoints de la API

### Autenticación (públicos)

| Método | Endpoint              | Descripción               |
|--------|-----------------------|---------------------------|
| POST   | `/api/auth/register`  | Registrar nuevo usuario   |
| POST   | `/api/auth/login`     | Iniciar sesión → JWT      |

### Herramientas

| Método | Endpoint           | Acceso            | Descripción                  |
|--------|--------------------|-------------------|------------------------------|
| GET    | `/api/tools`       | Público           | Listar todo el catálogo      |
| GET    | `/api/tools?available=true` | Público | Solo herramientas disponibles |
| POST   | `/api/tools`       | 🔒 Solo Owner     | Crear herramienta            |
| PUT    | `/api/tools/:id`   | 🔒 Solo Owner     | Actualizar herramienta       |
| DELETE | `/api/tools/:id`   | 🔒 Solo Owner     | Eliminar herramienta         |

---

## Ejemplos con curl

### Registrar un propietario
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Carlos Propietario",
    "email": "carlos@example.com",
    "password": "mipassword123",
    "role": "owner"
  }'
```

### Login
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "owner@example.com", "password": "password123"}'
```

Copia el `token` de la respuesta para usarlo en los siguientes requests.

### Ver catálogo (sin autenticación)
```bash
curl http://localhost:8080/api/tools
```

### Crear herramienta (requiere token de owner)
```bash
curl -X POST http://localhost:8080/api/tools \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TU_TOKEN_AQUI" \
  -d '{
    "name": "Martillo de Carpintero",
    "description": "Martillo de 500g con mango de fibra de vidrio",
    "category": "Manual",
    "is_available": true
  }'
```

### Actualizar herramienta
```bash
curl -X PUT http://localhost:8080/api/tools/UUID_DE_LA_HERRAMIENTA \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TU_TOKEN_AQUI" \
  -d '{"is_available": false}'
```

### Eliminar herramienta
```bash
curl -X DELETE http://localhost:8080/api/tools/UUID_DE_LA_HERRAMIENTA \
  -H "Authorization: Bearer TU_TOKEN_AQUI"
```

---

## Errores HTTP manejados

| Código | Cuándo ocurre                                      |
|--------|----------------------------------------------------|
| 400    | Body JSON inválido o campos requeridos faltantes   |
| 401    | Sin token, token inválido o credenciales incorrectas |
| 403    | Token válido pero rol sin permisos (no es owner)   |
| 404    | Herramienta no encontrada                          |
| 409    | Email ya registrado                                |
| 500    | Error interno del servidor (ver logs)              |

---

## Decisiones de diseño

- **pgx en lugar de GORM**: pgx es el driver nativo de PostgreSQL para Go, sin abstracciones innecesarias. Las queries SQL explícitas son más fáciles de optimizar y depurar que el ORM auto-generado.
- **pgxpool**: Un pool de conexiones reutiliza hasta 25 conexiones simultáneas en lugar de abrir una nueva por cada request HTTP.
- **Arquitectura en capas**: `handler → repository → database`. Los handlers no conocen SQL; los repositorios no conocen HTTP.
- **Error sentinela**: `ErrNotFound` y `ErrForbidden` permiten que el handler decida el código HTTP sin usar strings frágiles.
- **Graceful shutdown**: El servidor espera hasta 10 segundos a que terminen los requests activos antes de cerrarse (importante en producción con Docker/K8s).
