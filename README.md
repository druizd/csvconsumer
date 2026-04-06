# csvconsumer — SQL Consumer & Metrics API

> **Consumidor Linux** — Microservicio Go que ejecuta SQL en TimescaleDB vía RabbitMQ y expone una API de métricas en tiempo real.

---

## ¿Qué hace?

`csvconsumer` es el **punto final del pipeline de datos**. Escucha la cola `sql_execution_tasks` de RabbitMQ, ejecuta cada sentencia SQL recibida directamente contra TimescaleDB, y expone un endpoint HTTP `/stats` para monitorear el estado del sistema en tiempo real.

---

## Stack incluido (`docker-compose.yml`)

Este repositorio levanta **dos servicios juntos**:

| Servicio | Imagen | Puerto | Descripción |
|----------|--------|--------|-------------|
| `rabbitmq` | `rabbitmq:3-management` | `5672` / `15672` | Broker de mensajería — recibe los SQL del Windows sender |
| `consumer` | Build local (Go) | `8080` | Aplicación Go — consume RabbitMQ y ejecuta SQL en TimescaleDB |

> ⚠️ **Prerequisito**: Tener la base de datos TimescaleDB corriendo (ver repo `db-infra`) antes de levantar este stack.

---

## Levantamiento rápido

> Requiere **Docker Engine moderno** con el plugin Compose v2 (`docker compose` sin guión).

```bash
# Clonar el repo (primera vez)
git clone https://github.com/druizd/csvconsumer
cd csvconsumer

# Levantar RabbitMQ + consumer juntos
sudo docker compose up --build -d

# Verificar que ambos corren
sudo docker compose ps

# Ver logs en vivo
sudo docker compose logs -f
```

---

## Actualizar a la última versión

```bash
cd ~/csvconsumer

# Descartar cambios locales y traer la versión de GitHub
git stash
git pull origin main

# Reconstruir y reiniciar
sudo docker compose up --build -d
```

---

## API de Métricas (`GET /stats`)

```
http://<IP-SERVIDOR>:8080/stats
```

```json
{
  "archivos_fallidos":   0,
  "archivos_procesados": 142,
  "promedio_proceso_ms": 12,
  "status":              "UP",
  "tiempo_maximo_ms":    87,
  "uptime":              "4h32m10s",
  "estado_rabbitmq":     "CONNECTED",
  "estado_windows":      "UP"
}
```

| Campo | Descripción |
|-------|-------------|
| `estado_rabbitmq` | `CONNECTED` cuando hay conexión activa. `ERROR: ...` si falla. |
| `estado_windows` | `UP` si recibió heartbeat del Windows sender en los últimos 15s. `DOWN` si no. |

---

## Management UI de RabbitMQ

```
http://<IP-SERVIDOR>:15672
Usuario: shipper
Password: shipper123
```

Desde la UI puedes ver mensajes encolados, consumers activos y tasa de throughput en tiempo real.

---

## CI / CD — Auto-deploy desde GitHub Actions

Cada `push` a la rama `main` dispara automáticamente el pipeline que:
1. Se conecta al servidor Linux por SSH
2. Ejecuta `git pull origin main`
3. Reconstruye y reinicia los contenedores con `docker compose up --build -d`

Ver estado del pipeline: `https://github.com/druizd/csvconsumer/actions`

> **Secrets requeridos en GitHub** (Settings → Secrets and variables → Actions):
> - `SSH_PRIVATE_KEY` — clave privada SSH del servidor
> - `SSH_HOST` — IP del servidor (`13.86.114.224`)
> - `SSH_USER` — usuario (`azureuser`)

---

## Variables de entorno del consumer

| Variable | Default | Descripción |
|----------|---------|-------------|
| `RABBITMQ_URL` | `amqp://shipper:shipper123@localhost:5672/` | URL de RabbitMQ (mismo host) |
| `DATABASE_URL` | `postgres://postgres:postgres123@localhost:5432/logmetrics?sslmode=disable` | URL de TimescaleDB |
| `PORT` | `8080` | Puerto del servidor HTTP de métricas |

---

## Colas RabbitMQ

| Cola | Durable | Descripción |
|------|---------|-------------|
| `sql_execution_tasks` | ✅ Sí | SQL entrantes para ejecutar en TimescaleDB |
| `status_queue` | ❌ No | Heartbeats del Windows sender (cada 5s) |

---

## Repositorios relacionados

| Repositorio | Relación |
|-------------|----------|
| [`db-infra`](../db-infra/) | Provee TimescaleDB — debe levantarse primero |
| [`csvprocessor`](../csvprocessor/) | Genera los SQL en Windows |
| [`csvshipper-win`](../csvshipper-win/) | Envía los SQL a RabbitMQ desde Windows |
