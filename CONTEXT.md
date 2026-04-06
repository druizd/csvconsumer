# csvconsumer — Contexto del Repositorio

## ¿Qué es?

Servicio **Linux** que actúa como consumidor del sistema de procesamiento distribuido de logs.  
Corre como contenedor Docker y tiene dos responsabilidades principales:

1. **Consumir mensajes de RabbitMQ** → Recibe sentencias SQL listas para insertar y las ejecuta contra TimescaleDB.
2. **Exponer una API HTTP de métricas** → Permite monitorear el estado del sistema en tiempo real desde cualquier cliente externo (puerto 8080).

## Posición en el sistema

```
[csvprocessor (Win)] ──SQL→ [RabbitMQ] ──SQL→ [csvconsumer (Linux)] ──INSERT→ [TimescaleDB]
[csvshipper-win (Win)] ─────────────────────────────────────────────────────────────────────^
                  ↑ ambos publican en la misma cola sql_execution_tasks
```

## Stack técnico

| Componente | Detalle |
|------------|---------|
| Lenguaje | Go 1.25 |
| Base de datos | TimescaleDB (PostgreSQL) vía driver `lib/pq` |
| Mensajería | RabbitMQ vía `amqp091-go` |
| Despliegue | Docker + Docker Compose v2 |
| API interna | `net/http` estándar de Go |

## Servicios en el `docker-compose.yml`

El compose levanta **dos servicios juntos** (ya no es necesario instalar RabbitMQ por separado):

| Servicio | Descripción |
|----------|-------------|
| `rabbitmq` | Broker con imagen `rabbitmq:3-management`. Puertos `5672` (AMQP) y `15672` (Management UI). Usuario/pass: `shipper/shipper123`. Tiene healthcheck. |
| `consumer` | Aplicación Go compilada con el Dockerfile local. Depende de que `rabbitmq` esté healthy antes de arrancar. Usa `network_mode: host` para alcanzar tanto RabbitMQ como TimescaleDB en el host. |

## Estructura del código (`main.go`)

| Elemento | Descripción |
|----------|-------------|
| `Metrics` (struct) | Almacena contadores thread-safe: archivos procesados, fallidos, tiempos, estado de RabbitMQ y estado del Windows sender |
| `connectDB()` | Conecta a TimescaleDB con reintentos (5 intentos, 3s entre cada uno) |
| `runConsumer()` | Loop principal: se conecta a RabbitMQ, escucha `sql_execution_tasks` y ejecuta cada SQL recibido |
| `serveAPI()` | Levanta un servidor HTTP en el puerto 8080 con el endpoint `/stats` |
| `main()` | Lanza la API en goroutine, conecta a DB y entra en loop de reconexión a RabbitMQ |

## Colas RabbitMQ utilizadas

| Cola | Durable | Propósito |
|------|---------|-----------|
| `sql_execution_tasks` | ✅ Sí | Recibe los SQL a ejecutar (manual-ack) |
| `status_queue` | ❌ No | Recibe heartbeats del Windows sender (cada 5s) |

## API de Métricas (`GET /stats`)

```json
{
  "archivos_fallidos": 0,
  "archivos_procesados": 142,
  "promedio_proceso_ms": 12,
  "status": "UP",
  "tiempo_maximo_ms": 87,
  "uptime": "4h32m10s",
  "estado_rabbitmq": "CONNECTED",
  "estado_windows": "UP"
}
```

> `estado_windows` cambia a `"DOWN"` si no se recibe ningún heartbeat de Windows en los últimos **15 segundos**.

## Variables de entorno

| Variable | Default | Descripción |
|----------|---------|-------------|
| `RABBITMQ_URL` | `amqp://shipper:shipper123@localhost:5672/` | Apunta a localhost — RabbitMQ corre en el mismo host |
| `DATABASE_URL` | `postgres://postgres:postgres123@localhost:5432/logmetrics?sslmode=disable` | URL de TimescaleDB |
| `PORT` | `8080` | Puerto del servidor HTTP de métricas |

## CI/CD — GitHub Actions

El repositorio tiene un pipeline de auto-deploy en `.github/workflows/deploy.yml`:
- Se dispara en cada `push` a `main`
- Se conecta al servidor Linux por SSH
- Ejecuta `git pull origin main` + `docker compose up --build -d`
- Requiere 3 Secrets en GitHub: `SSH_PRIVATE_KEY`, `SSH_HOST`, `SSH_USER`

## Notas de despliegue

- Requiere **Docker Engine moderno** (plugin Compose v2) — usar `docker compose` **sin guión**
- La versión antigua `docker-compose` (Python, v1.29.x) tiene un bug con `ContainerConfig` al recrear contenedores
- Para resetear estado local en el servidor antes de hacer `git pull`: `git stash`

## Repositorios relacionados

| Repositorio | Relación |
|-------------|----------|
| `db-infra` | Provee la instancia de TimescaleDB que este servicio consume |
| `csvprocessor` | Genera los SQL que este servicio ejecuta (vía RabbitMQ) |
| `csvshipper-win` | Servicio Windows que también envía SQL a la misma cola |
