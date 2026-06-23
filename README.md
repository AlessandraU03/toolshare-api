# ToolShare Backend API 🔧

API RESTful en Go estructurada bajo **Arquitectura Hexagonal Vertical Slicing**, diseñada para la gestión de inventario, préstamo de herramientas de construcción con apretón de manos digital transaccional, y valuación automática con Inteligencia Artificial.

Construida con **Gin**, **pgx/v5** y **JWT**.

---

## Arquitectura y Flujo Completo de la Aplicación

El ecosistema de **ToolShare** está compuesto por tres componentes principales:
1. **Frontend (Flutter)**: Aplicación móvil que interactúa con el usuario final, captura coordenadas GPS reales y toma fotografías del desgaste físico de las herramientas.
2. **Backend (Go API)**: Orquestador transaccional y pasarela de seguridad. Administra la lógica de negocio, persiste información en PostgreSQL, valida suscripciones Pro y actúa de proxy seguro con el microservicio de IA.
3. **Microservicio de ML (Python - FastAPI)**: Contiene el modelo de clasificación de imágenes (Random Forest/Redes Neuronales) para determinar el nivel de desgaste físico de la herramienta y realiza web scraping en tiempo real de Mercado Libre para valuar y sugerir precios justos de renta.

```mermaid
graph TD
    Flutter[App Flutter] -->|POST /auth/register| GoAPI[Backend Go API :8080]
    Flutter -->|POST /tools/predict-condition| GoAPI
    Flutter -->|GET /tools/auto-valuate| GoAPI
    GoAPI -->|Proxy POST /predict-condition| PythonML[Microservicio ML Python :8000]
    GoAPI -->|Proxy GET /auto-valuate| PythonML
    PythonML -->|Scraping / API| MercadoLibre[Mercado Libre API]
    GoAPI -->|Queries SQL| PostgreSQL[(PostgreSQL :5432)]
```

---

## Estructura del Proyecto (Slicing Vertical)

```
Api_Apptransacional/
├── cmd/
│   └── server/
│       └── main.go              # Punto de entrada (inicialización y Graceful Shutdown)
├── internal/
│   ├── tool/                    # Slice Vertical de Herramientas
│   │   ├── domain/              # Modelos de dominio lógicos (Tool, GPS)
│   │   ├── ports/               # Interfaces de servicios y repositorios (Puertos)
│   │   ├── postgres/            # Implementación física del repositorio SQL
│   │   ├── service/             # Lógica de negocio (Autovaluación, ML Proxy)
│   │   └── handler/             # Controladores Gin y DTOs de Request/Response
│   ├── user/                    # Slice Vertical de Usuarios (Registro, Login, Planes)
│   ├── rental/                  # Slice Vertical de Alquileres (Contratos, Handshake)
│   ├── shared/                  # Utilidades compartidas (Middleware de Auth, Router, DB Pool)
├── migrations/
│   └── schema.sql               # Script unificado de base de datos
├── uploads/                     # Carpeta local para almacenar imágenes de herramientas
├── .env.example                 # Variables de entorno de plantilla
└── README.md
```

---

## Flujos Clave de Negocio

### 1. Registro e Identificación Segura
* El registro de usuarios (`/api/auth/register`) requiere obligatoriamente **Nombre**, **Correo**, **Contraseña**, **Teléfono** (10 dígitos) y **Clave de Elector / INE**.
* Las contraseñas se almacenan de forma segura utilizando encriptación unidireccional con **bcrypt** (cost = 12).

### 2. Clasificación de Desgaste por Fotografía (ML)
* El usuario toma una foto de la herramienta desde Flutter. La app envía la imagen binaria (`multipart/form-data`) al endpoint de Go `/api/tools/predict-condition`.
* El backend de Go actúa de intermediario y reenvía el archivo al microservicio de Python (`:8000/predict-condition`), el cual responde con la clase predicha (`nuevo`, `uso_moderado` o `viejo_desgastado`) y su puntaje de confianza.
* El frontend de Flutter recibe esta clasificación y selecciona automáticamente el nivel de condición física en el formulario (`Nuevo`, `Buen Estado` o `Desgastado`).

### 3. Valuación y Sugerencia de Precios (Mercado Libre)
* Al ingresar el nombre, marca, modelo y categoría de la herramienta, se dispara una petición a `/api/tools/auto-valuate`.
* El microservicio de Python realiza búsquedas en Mercado Libre, procesa los precios con modelos de regresión y calcula:
  * **Valor Estimado de Catálogo** (`estimated_value`): El precio promedio actual del producto en el mercado.
  * **Tarifa de Renta Diaria Sugerida** (`suggested_daily_rate`): Tarifa recomendada calculada mediante IA.
  * **Precio Mínimo de Renta** (`minimum_daily_rate`): Tarifa mínima correspondiente al 50% de recuperación del valor estimado prorrateado a 30 días.
* Si el **Valor Estimado** de la herramienta supera los **$1,500 MXN**, el backend de Go restringe la publicación solo a usuarios con plan **Pro** activo (`is_pro = true`).

### 4. Apretón de Manos Digital Transaccional (Handshake)
El flujo de alquiler de herramientas es un contrato de mutua aceptación digital:
1. **Solicitud (`pending`)**: El solicitante pide una herramienta. Se realiza una pre-autorización de fondos (garantía) mediante Mercado Pago.
2. **Entrega (`active`)**: Tanto el propietario como el solicitante deben confirmar la entrega física desde sus aplicaciones. Al completarse la doble confirmación, el servidor genera un **hash SHA-256** del contrato legal inmutable y captura las **coordenadas GPS** exactas de la entrega. El estado pasa a `active`.
3. **Devolución / Disputa (`completed` / `disputed`)**: 
   * Si el propietario confirma que la herramienta regresó en buen estado, se liberan los fondos de garantía y la transacción pasa a `completed`.
   * Si hay inconformidad, la transacción entra en estado de `disputed` (arbitraje) reteniendo la garantía para cubrir reposición.

---

## Instalación y Configuración del Servidor

### Requisitos Previos
- **Go 1.22** o superior instalado en la computadora.
- **PostgreSQL 14** o superior corriendo localmente.
- **Python 3.10+** (para correr el microservicio de IA localmente en el puerto `8000`).

### 1. Inicializar la Base de Datos
Abre tu consola de PostgreSQL y ejecuta:
```sql
CREATE DATABASE tool_inventory;
```
Aplica el script unificado de base de datos ubicado en la carpeta de migraciones:
```bash
psql -U postgres -d tool_inventory -f migrations/schema.sql
```

### 2. Configurar Variables de Entorno
Copia el archivo `.env.example` a `.env` en la raíz de `Api_Apptransacional`:
```bash
cp .env.example .env
```
Asegúrate de editar tu `.env` con tus credenciales de PostgreSQL:
```env
SERVER_PORT=8080
DATABASE_URL=postgres://tu_usuario:tu_contraseña@localhost:5432/tool_inventory?sslmode=disable
JWT_SECRET=UnAcAdEnAaLeAtOrIaYSeGuRaDeMaYaSDe32ChArS
JWT_EXPIRATION=24h
```

### 3. Compilar y Ejecutar el Servidor Go
Instala las dependencias y compila el ejecutable estático:
```bash
go mod tidy
go build -o server.exe cmd/server/main.go
```
Para iniciar el servidor, ejecuta:
```bash
.\server.exe
```
El servidor arrancará e imprimirá en consola:
```
2026/06/19 12:24:43 Conexión a PostgreSQL establecida
2026/06/19 12:24:43 Servidor en http://localhost:8080
```

---

## Conexión de Dispositivos Móviles Físicos (USB ADB)

Cuando usas un celular físico conectado a tu computadora por USB para probar la aplicación en Flutter, los cortafuegos y el aislamiento de red (AP Isolation) de tu router Wi-Fi bloquearán las peticiones REST. 

Para solucionar esto de manera robusta y sin configurar IPs variables:

### 1. Activar Depuración por USB en el Celular
Ve a **Ajustes > Opciones de desarrollador** en tu teléfono Android y activa la **Depuración por USB**.

### 2. Ejecutar Redirección de Puertos ADB
Abre una terminal en tu computadora y ejecuta la utilidad ADB del SDK de Android para enlazar el puerto local del celular con el de la computadora:
```powershell
& "C:\Users\aless\AppData\Local\Android\Sdk\platform-tools\adb.exe" reverse tcp:8080 tcp:8080
```

### 3. Configuración en la App móvil (Flutter)
La app móvil está configurada en `lib/shared/config/api_config.dart` para apuntar a `127.0.0.1:8080`. Gracias a `adb reverse`, toda la comunicación viajará de manera segura y veloz a través del cable USB directo a tu servidor local de Go.
