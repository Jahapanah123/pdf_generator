# PDF Generator Service

A production-style asynchronous PDF generation system built with Go, PostgreSQL, RabbitMQ, and Docker.

---

##  Overview

This service processes PDF generation requests asynchronously using a message queue and worker pool architecture.

---

##  Architecture

Flow:

Client → API → PostgreSQL → RabbitMQ → Worker → PDF Generation → DB Update → Client (SSE)

---

##  Features

- Asynchronous job processing
- Worker pool with concurrency control
- Retry mechanism with Dead Letter Queue (DLQ)
- Real-time updates via Server-Sent Events (SSE)
- JWT-based authentication
- Dockerized setup
- Structured logging
- Clean layered architecture

---

## Project Structure

cmd/ # entry point
internal/
handler/ # HTTP handlers
service/ # business logic
repository/ # DB layer
worker/ # background workers
pkg/ # shared utilities


---

## Environment Variables

Create a `.env` file:

DB_URL=postgres://user:password@db:5432/pdf_db
RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/
JWT_SECRET=your_secret_key


---

## API Endpoints

POST /jobs -> create PDF job
GET /jobs/{id} -> get job status
GET /stream/{id} -> real-time updates (SSE)


---

## How It Works

1. Client sends a PDF generation request
2. API stores job metadata in PostgreSQL
3. Job message is published to RabbitMQ
4. Worker consumes the job
5. PDF is generated
6. Job status is updated in DB
7. Client receives updates via SSE

---

## Retry & Dead Letter Queue

- Failed jobs are retried up to `N` times
- After max retries, job is moved to DLQ
- Prevents data loss and enables debugging of failed jobs

---

##  Running Locally

```bash
mkdir output
docker-compose up --build

##  Future Improvements
Kubernetes deployment
CI/CD pipeline
Monitoring with Prometheus & Grafana
Distributed tracing


