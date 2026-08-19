# Marketplace Workflow Platform

A production-style Kubernetes workflow orchestration project built with **Go, Argo Workflows, Kubernetes, Kubebuilder, controller-runtime, client-go, and gRPC**.

The project demonstrates two different Kubernetes controller patterns:

1. **Marketplace Workflow Controller** — manages desired workflow scheduling.
2. **Marketplace Publishing Status Controller** — watches Argo Workflow executions and synchronizes workflow/step status to the Marketplace Publishing Service.

---

## Architecture

```text
                         CONTROL PLANE
                         =============

MarketplaceWorkflow CRD
         |
         | watch / reconcile
         v
Marketplace Workflow Controller
         |
         | Create / Update
         v
Argo CronWorkflow
         |
         | schedule fires
         v


                         EXECUTION PLANE
                         ===============

Argo Workflow
      |
      +---- discover
      |
      +---- validate
      |
      +---- publish
      |
      v
Go CLI Worker
      |
      +---- Marketplace Publishing APIs
      +---- Publisher APIs
      +---- OCI internal service APIs


                         STATUS PLANE
                         ============

Argo Workflow.status
         |
         | Kubernetes LIST / WATCH
         v
Shared Informer
         |
         v
Informer Cache
         |
         | Add / Update / Delete event
         v
Typed Rate-Limiting WorkQueue
         |
         | namespace/workflow-name
         v
Worker Goroutines
         |
         v
processNextWorkItem()
         |
         v
syncWorkflow()
         |
         | read latest Workflow from cache
         v
JobStatus Mapper
         |
         | workflow + logical step status
         v
Marketplace Publishing gRPC Client
         |
         v
Marketplace Publishing Service
         |
         v
In-Memory Job Store
         |
         | Future phase
         v
OCI NoSQL
TTL: 30-60 days
```

### Workflow state

Argo remains the source of truth for current workflow execution.

```text
Argo Workflow
   |
   +-- phase
   +-- startedAt
   +-- finishedAt
   +-- message
   |
   +-- nodes
        |
        +-- discover
        +-- validate
        +-- publish
        +-- retry attempts
```

The status controller converts Argo's internal node structure into a simpler Marketplace Publishing representation.

For example:

```text
Argo

validate Retry
   |
   +-- attempt 1 FAILED
   +-- attempt 2 FAILED
   +-- attempt 3 SUCCEEDED

              ↓

Marketplace Publishing

validate
status   = SUCCEEDED
attempts = 3
```

The controller sends the **latest complete workflow snapshot** instead of trying to replay every Kubernetes event.

---

## Repository Structure

```text
marketplace-workflow-platform/
│
├── argo/
│   ├── templates/
│   └── cron/
│
├── worker/
│   └── Go CLI commands executed by Argo
│
├── controller/
│   └── MarketplaceWorkflow scheduling controller
│
└── publishing-status-controller/
    │
    ├── api/
    │   ├── publishing.proto
    │   ├── publishing.pb.go
    │   └── publishing_grpc.pb.go
    │
    ├── cmd/
    │   ├── controller/
    │   │   └── main.go
    │   │
    │   └── publishing-service/
    │       └── main.go
    │
    └── internal/
        ├── controller/
        │   ├── controller.go
        │   └── mapper.go
        │
        ├── model/
        │   └── status.go
        │
        └── publishing/
            ├── client.go
            ├── mapper.go
            └── service.go
```

---

# Running Locally

The examples assume:

- Docker Desktop is running
- Kind cluster is running
- Argo Workflows is installed
- Kubernetes context is configured
- namespace is `argo`

Verify the cluster:

```bash
kubectl cluster-info
```

Verify Argo:

```bash
kubectl get pods -n argo
```

Verify the publishing template:

```bash
kubectl get workflowtemplate marketplace-publishing -n argo
```

If needed, apply it:

```bash
kubectl apply -f argo/templates/publishing.yaml
```

---

## 1. Run Marketplace Publishing Service

Open terminal 1:

```bash
cd publishing-status-controller

go run ./cmd/publishing-service
```

Expected:

```text
Marketplace Publishing Service listening on :50051
```

The service currently stores workflow status in memory.

A future phase will replace the in-memory repository with OCI NoSQL using a 30-60 day TTL.

---

## 2. Run Marketplace Publishing Status Controller

Open terminal 2:

```bash
cd publishing-status-controller

go run ./cmd/controller
```

The controller performs:

```text
Workflow event
      ↓
Informer
      ↓
TypedRateLimitingQueue
      ↓
Worker
      ↓
syncWorkflow()
      ↓
Informer Cache
      ↓
JobStatus
      ↓
gRPC
      ↓
Marketplace Publishing Service
```

Existing Workflows may generate `ADD` events when the informer initially synchronizes its cache.

---

# Success Scenario

Open terminal 3.

Submit the publishing workflow:

```bash
argo submit \
  --from workflowtemplate/marketplace-publishing \
  -n argo \
  -p failureMode=success
```

Watch the workflow:

```bash
argo list -n argo
```

or:

```bash
argo get <workflow-name> -n argo
```

The status controller should observe transitions such as:

```text
PENDING
   ↓
RUNNING
   ↓
SUCCEEDED
```

The Marketplace Publishing Service will receive snapshots similar to:

```text
workflow = marketplace-publishing-xxxxx
phase    = SUCCEEDED

discover  SUCCEEDED
validate  SUCCEEDED
publish   SUCCEEDED
```

---

# Workflow Retry Scenario

Run:

```bash
argo submit \
  --from workflowtemplate/marketplace-publishing \
  -n argo \
  -p failureMode=transient
```

The validation step intentionally fails transiently before succeeding.

Argo may execute:

```text
validate
   |
   +-- attempt 1 FAILED
   +-- attempt 2 FAILED
   +-- attempt 3 SUCCEEDED
```

The Marketplace Publishing status controller exposes this as one logical step:

```text
validate
phase    = SUCCEEDED
attempts = 3
```

This prevents individual retry failures from incorrectly being treated as final workflow failures.

---

# Publishing Service Failure / Controller Retry Scenario

This demonstrates why the controller uses a **Typed Rate-Limiting WorkQueue**.

First keep the status controller running.

Stop the Marketplace Publishing Service:

```text
Ctrl+C
```

Then submit another workflow:

```bash
argo submit \
  --from workflowtemplate/marketplace-publishing \
  -n argo \
  -p failureMode=success
```

The workflow continues running in Argo, but status synchronization fails:

```text
Workflow
   ↓
Status Controller
   ↓
gRPC
   X
Marketplace Publishing unavailable
```

The controller returns an error and requeues the workflow using:

```text
AddRateLimited(key)
```

Retries use exponential backoff approximately like:

```text
1 second
2 seconds
4 seconds
8 seconds
16 seconds
...
```

Restart the service:

```bash
go run ./cmd/publishing-service
```

A subsequent retry reads the **latest Workflow state from the informer cache** and successfully synchronizes it.

```text
Marketplace Publishing unavailable
          ↓
AddRateLimited()
          ↓
retry with backoff
          ↓
Marketplace Publishing recovers
          ↓
sync latest Workflow
          ↓
success
          ↓
queue.Forget(key)
```

---

## Current Persistence

Current/recent workflow execution state:

```text
Argo Workflow
      ↓
Kubernetes API
      ↓
etcd
```

Marketplace Publishing history currently uses an in-memory store.

Future production persistence:

```text
Marketplace Publishing Service
          ↓
OCI NoSQL
          ↓
workflowUID -> JobStatus
          ↓
TTL 30-60 days
```

Argo/Kubernetes remains the source of truth for active execution. OCI NoSQL will provide longer-lived Marketplace operational history.