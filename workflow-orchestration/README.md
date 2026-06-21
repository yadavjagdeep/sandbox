# Workflow Orchestration with Apache Airflow

A hands-on demo of workflow management using Apache Airflow — running fully local with Docker. Implements an order processing pipeline to understand DAGs, retries, branching, and task dependencies.

## What Is a Workflow Management Tool?

A system that orchestrates **multi-step processes** where:
- Steps execute in a defined order (or parallel with a join)
- Failed steps get retried automatically with backoff
- You need visibility into what step things are stuck at
- Conditional branching handles success/failure paths differently

Examples: Apache Airflow, AWS Step Functions, Temporal, Prefect.

## Why Not Just Use Kafka?

Kafka is for **event streaming** (fire-and-forget, decoupled). Workflow tools are for **orchestration** (ordered steps, retries, visibility).

| Need | Kafka | Workflow Tool |
|------|-------|--------------|
| Step ordering (A → B → C) | Wire manually across topics | Define a DAG, engine handles it |
| Retry on failure | Build retry logic per consumer | Config: `retries=3, delay=10s` |
| Conditional branching | Complex routing in code | Built-in branching operators |
| "Where is this stuck?" | Grep logs across services | Visual UI with task states |
| Wait then execute | Hack with delayed messages | Built-in timers/sensors |

**Kafka = pipes. Workflow tools = plumbing diagram + valves + gauges + repair manual.**

## When to Use a Workflow Tool

- ETL/data pipelines (extract → transform → load)
- Order fulfillment (validate → reserve → ship → notify)
- Onboarding flows (create account → send email → provision resources)
- CI/CD pipelines (build → test → deploy → smoke test)
- Payment processing (authorize → capture → settle → reconcile)
- Any process where steps have dependencies, retries matter, and you need observability

## The Demo: Order Processing Pipeline

```
start → validate_payment → reserve_inventory → process_shipment → notify_customer → end_success
              ↓ (fails after 3 retries)
        handle_payment_failure → end_failure
```

- **validate_payment** — 80% success rate, retries 3x with 5s delay
- **reserve_inventory** — 90% success rate, retries 2x with 10s delay
- **process_shipment** — generates tracking ID
- **notify_customer** — sends notification
- **handle_payment_failure** — refund + notify (runs only if payment fails)

## Key Airflow Concepts

**DAG (Directed Acyclic Graph)** — the workflow definition. Tasks are nodes, dependencies are edges.

**Operators** — task types. `PythonOperator` runs Python functions. Others: `BashOperator`, `HttpOperator`, sensors.

**Retries** — per-task: `retries=3, retry_delay=timedelta(seconds=5)`.

**Trigger Rules** — when should a task run?
- `all_success` (default): all upstream succeeded
- `one_failed`: any upstream failed (for error handlers)

**XCom** — data passing between tasks (return value from one task, pull in next).

## Running Locally

### Prerequisites
- Docker + docker-compose

### Start

```bash
cd workflow-orchestration
sudo docker-compose up -d
```

Wait ~30 seconds for Airflow to initialize.

### Access the UI

Open http://localhost:8080

Login: `admin` / `admin`

### Trigger a Run

Option 1 — from the UI:
- Click on `order_processing_pipeline`
- Click the Play button (▶) → "Trigger DAG with config"
- Enter: `{"order_id": "ORD-12345"}`

Option 2 — from terminal:
```bash
sudo docker-compose exec airflow-scheduler \
  airflow dags trigger order_processing_pipeline \
  --conf '{"order_id": "ORD-12345"}'
```

### Watch It Execute

In the UI, click the DAG → switch to "Graph" view:
- Green = success
- Yellow = running
- Red = failed
- Orange = up for retry

Click any task to see its logs.

### Test Retries

Trigger multiple runs. With 20% payment failure rate, some runs will:
1. Fail at `validate_payment`
2. Retry 3 times (visible in task instance details)
3. If all retries fail → `handle_payment_failure` executes

## Project Structure

```
workflow-orchestration/
├── docker-compose.yaml    # Airflow infra (webserver, scheduler, postgres)
├── dags/
│   └── order_pipeline.py  # The DAG definition
├── .env                   # Airflow user config
└── README.md
```

## Cleanup

```bash
sudo docker-compose down -v
```
