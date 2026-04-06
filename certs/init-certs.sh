#!/bin/bash
# =============================================================================
# init-certs.sh — Entrypoint para RabbitMQ con TLS auto-generado
# Genera un certificado autofirmado si no existe, configura rabbitmq.conf
# con TLS habilitado en puerto 5671, y arranca RabbitMQ.
# =============================================================================
set -e

CERT_DIR="/certs"
CERT_FILE="$CERT_DIR/server.crt"
KEY_FILE="$CERT_DIR/server.key"
RABBITMQ_CONF="/etc/rabbitmq/rabbitmq.conf"

if [ ! -f "$CERT_FILE" ] || [ ! -f "$KEY_FILE" ]; then
    echo "[init-certs] Generando certificado TLS autofirmado para RabbitMQ (RSA 4096, 10 años)..."
    mkdir -p "$CERT_DIR"
    openssl req -new -x509 -days 3650 -nodes \
        -newkey rsa:4096 \
        -keyout "$KEY_FILE" \
        -out "$CERT_FILE" \
        -subj "/C=CL/ST=Santiago/L=Santiago/O=EmeCloud/CN=rabbitmq-local"
    chmod 600 "$KEY_FILE"
    chmod 644 "$CERT_FILE"
    echo "[init-certs] Certificado RabbitMQ generado exitosamente."
else
    echo "[init-certs] Certificado existente encontrado, reutilizando."
fi

# Escribir configuración TLS de RabbitMQ
cat > "$RABBITMQ_CONF" <<EOF
# Puerto AMQP sin TLS (deshabilitado — solo se usa internamente por el consumer)
listeners.tcp.default = 5672

# Puerto AMQPS con TLS
listeners.ssl.default = 5671
ssl_options.certfile   = $CERT_FILE
ssl_options.keyfile    = $KEY_FILE
ssl_options.verify     = verify_none
ssl_options.fail_if_no_peer_cert = false

# Management plugin (dashboard web)
management.tcp.port = 15672
EOF

echo "[init-certs] rabbitmq.conf escrito con TLS habilitado en puerto 5671."

# Arrancar RabbitMQ
exec docker-entrypoint.sh rabbitmq-server
