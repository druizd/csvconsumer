package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Metrics struct {
	ArchivosFallidos  int64  `json:"archivos_fallidos"`
	ArchivosProcesados int64  `json:"archivos_procesados"`
	PromedioProcesoMs int64  `json:"promedio_proceso_ms"`
	Status            string `json:"status"`
	TiempoMaximoMs    int64  `json:"tiempo_maximo_ms"`
	Uptime            string `json:"uptime"`
	EstadoRabbitMQ    string `json:"estado_rabbitmq"`
	EstadoWindows     string `json:"estado_windows"`

	// Internal fields
	startTime       time.Time
	lastWindowsPing time.Time
	totalProcesoMs  int64
	mu              sync.Mutex
}

func (m *Metrics) GetSnapshot() Metrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Calculate logical fields
	snap := *m
	snap.Uptime = time.Since(m.startTime).Round(time.Second).String()

	if time.Since(m.lastWindowsPing) > 15*time.Second {
		snap.EstadoWindows = "DOWN"
	} else {
		snap.EstadoWindows = "UP"
	}

	if snap.totalProcesoMs > 0 && snap.ArchivosProcesados > 0 {
		snap.PromedioProcesoMs = snap.totalProcesoMs / snap.ArchivosProcesados
	}

	return snap
}

func (m *Metrics) RecordSuccess(durationMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ArchivosProcesados++
	m.totalProcesoMs += durationMs
	if durationMs > m.TiempoMaximoMs {
		m.TiempoMaximoMs = durationMs
	}
}

func (m *Metrics) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ArchivosFallidos++
}

func (m *Metrics) RecordWindowsPing() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastWindowsPing = time.Now()
}

func (m *Metrics) SetRabbitStatus(s string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EstadoRabbitMQ = s
}

func connectDB() *sql.DB {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres123@db:5432/logmetrics?sslmode=disable"
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}

	// Try pinging
	for i := 0; i < 5; i++ {
		if err := db.Ping(); err == nil {
			log.Println("Successfully connected to TimescaleDB")
			return db
		}
		log.Printf("Waiting for DB connection... %d/5", i+1)
		time.Sleep(3 * time.Second)
	}
	log.Fatal("Could not connect to DB after 5 attempts")
	return nil
}

func main() {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	metrics := &Metrics{
		Status:         "UP",
		EstadoRabbitMQ: "CONNECTING",
		EstadoWindows:  "UNKNOWN",
		startTime:      time.Now(),
	}

	go serveAPI(metrics)

	db := connectDB()
	defer db.Close()

	for {
		err := runConsumer(rabbitURL, db, metrics)
		if err != nil {
			metrics.SetRabbitStatus(fmt.Sprintf("ERROR: %v", err))
			log.Printf("RabbitMQ Consumer error: %v. Reconnecting in 5s...", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func serveAPI(metrics *Metrics) {
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		snap := metrics.GetSnapshot()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snap)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting metrics API on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func runConsumer(rabbitURL string, db *sql.DB, metrics *Metrics) error {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	metrics.SetRabbitStatus("CONNECTED")

	q, err := ch.QueueDeclare("sql_execution_tasks", true, false, false, false, nil)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	sq, err := ch.QueueDeclare("status_queue", false, false, false, false, nil)
	if err != nil {
		return err
	}
	statusMsgs, err := ch.Consume(sq.Name, "", true, false, false, false, nil)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Status worker
	go func() {
		for d := range statusMsgs {
			var msg map[string]string
			if err := json.Unmarshal(d.Body, &msg); err == nil && msg["os"] == "windows" {
				metrics.RecordWindowsPing()
			}
		}
	}()

	log.Println("Waiting for messages...")
	for d := range msgs {
		startTime := time.Now()
		
		sqlStmt := string(d.Body)
		_, errExec := db.ExecContext(ctx, sqlStmt)

		duration := time.Since(startTime).Milliseconds()

		if errExec != nil {
			metrics.RecordError()
			if d.ReplyTo != "" {
				ch.PublishWithContext(ctx, "", d.ReplyTo, false, false, amqp.Publishing{
					CorrelationId: d.CorrelationId,
					Body:          []byte(fmt.Sprintf("ERROR: %v", errExec)),
				})
			}
			log.Printf("SQL Error: %v", errExec)
		} else {
			metrics.RecordSuccess(duration)
			if d.ReplyTo != "" {
				ch.PublishWithContext(ctx, "", d.ReplyTo, false, false, amqp.Publishing{
					CorrelationId: d.CorrelationId,
					Body:          []byte("SUCCESS"),
				})
			}
		}

		d.Ack(false)
	}

	metrics.SetRabbitStatus("DISCONNECTED")
	return fmt.Errorf("consumer channel closed")
}
