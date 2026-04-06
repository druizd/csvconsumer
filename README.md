# SQL Shipper - Go Consumer & API

Microservicio escrito en Golang para el sistema asíncrono de SQL.
Escucha las colas de RabbitMQ, inyecta las transacciones directamente sobre el repósitorio de TimescaleDB y provee un mini Dashboard sobre HTTP.

## Requisitos
- Servidor RabbitMQ
- Haber encendido primero el proyecto `database` (TimescaleDB).

## Levantamiento Rápido Automático
A nivel nativo puede compilarse o desplegarse enteramente con docker-compose:
```bash
docker-compose up --build -d
```
*(Expone tu API en http://localhost:8080/stats)*

## CI / CD Integrado
El pipeline de Github Actions vigila tanto la compilación cruzada estricta nativa de Golang 1.22 como la compilación exitosa y salud de la instancia de Docker resultabte para cada Pull Request.
