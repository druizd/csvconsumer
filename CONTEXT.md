# csvconsumer — Contexto del Repositorio

## ¿Qué es?

Servicio **Linux** que actúa como consumidor del sistema de procesamiento distribuido de logs.  
Corre como un contenedor Docker y tiene dos responsabilidades principales:

1. **Consumir mensajes de RabbitMQ** → Recibe sentencias SQL listas para insertar y las ejecuta contra la base de datos TimescaleDB.
2. **Exponer una API HTTP de métricas** → Permite monitorear el estado del servicio en tiempo real desde cualquier cliente externo (puerto 8080).

## Posición en el sistema

```
[csvprocessor (Win)] ──SQL─→ [RabbitMQ] ──SQL─→ [csvconsumer (Linux)] ──INSERT─→ [TimescaleDB]
                                  ↑
                         [csvshipper-win (Win)] ──SQL─→ [RabbitMQ] (flujo alternativo/adicional)
```

Este servicio es el **punto de entrada final** de los datos: toda la cadena de procesamiento termina aquí con la ejecución SQL contra la base de datos.

## Stack técnico

| Componente | Detalle |
|------------|---------|
| Lenguaje | Go 1.22 |
| Base de datos | TimescaleDB (PostgreSQL) vía driver `lib/pq` |
| Mensajería | RabbitMQ vía `amqp091-go` |
| Despliegue | Docker + Docker Compose |
| API interna | `net/http` estándar de Go |

## Estructura del código (`main.go`)

| Elemento | Descripción |
|----------|-------------|
| `Metrics` (struct) | Almacena contadores thread-safe: archivos procesados, fallidos, tiempos, estado de RabbitMQ y estado del Windows sender |
| `connectDB()` | Conecta a TimescaleDB con reintentos (5 intentos, 3s entre cada uno) |
| `runConsumer()` | Loop principal: se conecta a RabbitMQ, escucha la cola `sql_execution_tasks` y ejecuta cada SQL recibido |
| `serveAPI()` | Levanta un servidor HTTP en el puerto 8080 con el endpoint `/stats` |
| `main()` | Lanza la API en goroutine, conecta a DB y entra en loop de reconexión a RabbitMQ |

## Colas RabbitMQ utilizadas

| Cola | Durable | Propósito |
|------|---------|-----------|
| `sql_execution_tasks` | ✅ Sí | Recibe los SQL a ejecutar (consumidor manual-ack) |
| `status_queue` | ❌ No | Recibe heartbeats del servicio Windows para saber si está vivo |

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
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | URL de conexión a RabbitMQ |
| `DATABASE_URL` | `postgres://postgres:postgres123@db:5432/logmetrics?sslmode=disable` | URL de conexión a TimescaleDB |
| `PORT` | `8080` | Puerto del servidor HTTP de métricas |

## Despliegue

```bash
# Modo producción vía docker-compose
docker-compose up -d
```

La configuración de producción usa `network_mode: host` para que el contenedor acceda al puerto 5432 de TimescaleDB exportado directamente en el host Linux.

## Dependencias Go

- `github.com/lib/pq` — Driver PostgreSQL
- `github.com/rabbitmq/amqp091-go` — Cliente RabbitMQ

## Repositorios relacionados

| Repositorio | Relación |
|-------------|----------|
| `db-infra` | Provee la instancia de TimescaleDB que este servicio consume |
| `csvprocessor` | Genera los SQL que este servicio ejecuta (vía RabbitMQ) |
| `csvshipper-win` | Servicio Windows alternativo que también envía SQL a la misma cola |
