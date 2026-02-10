# Distributed Enrollment System

A fault-tolerant, distributed platform built with **Go**, **Docker**, and **Stateless JWT Authentication**.

## Overview

This system simulates a Service-Oriented Architecture (SOA) where services run on isolated networked nodes. Unlike a monolithic app, it demonstrates **Fault Tolerance** (Graceful Degradation) and **Distributed Security** across a custom software-defined network.

### Key Architecture

* **Portal (Edge Gateway):** The MVC Controller that aggregates data. It implements a **Circuit Breaker** pattern to handle backend failures gracefully.
* **Auth Service (IdP):** The Identity Provider that issues stateless JWT tickets.
* **Course Service (Catalog):** Manages course listings and atomic enrollment slots using in-memory concurrency controls.
* **Grade Service (Protected API):** A secured API that uses **Token Introspection** to validate requests dynamically against the IdP.

## System Design & Resilience

### The Circuit Breaker Pattern
To prevent cascading failures, the Portal wraps network calls in a timeout. If a backend service (like Grades) is unreachable, the Portal degrades functionality rather than crashing.

```text
[ Browser ]
    |
(HTTP/HTML)
    |
    v
[ Portal Gateway ] --(Timeout/Circuit Breaker)--> [ Grade Service ]
    |                                                  ^
    |                                                  |
    +----(Auth Token)-----> [ Auth Service ] <--(Introspection)--+

```

### Security Architecture (Introspection)

Instead of sharing a database, we use **Token Introspection** (RFC 7662 style) to validate trust.

1. **Request:** Client sends `Authorization: Bearer <token>` to Grade Node.
2. **Pause:** Grade Node makes a back-channel HTTP call to `GET /validate` on the Auth Node.
3. **Verify:** Auth Node validates the signature and returns the user's Role.
4. **Enforce:** Grade Node applies RBAC (Faculty vs. Student) based on the fresh response.

---

## Engineering Highlights

### 1. 98% Container Reduction

Utilized **Multi-Stage Docker Builds** to compile Go binaries and inject them into minimal Alpine Linux images.

* **Original Build:** ~800MB (Debian/Ubuntu base)
* **Optimized Build:** **~15MB** (Alpine + Binary)

### 2. Graceful Degradation (Fault Tolerance)

The system prioritizes **Availability** over Completeness.

* **Scenario:** The Grading Service crashes.
* **Outcome:** The User Dashboard continues to load. Course Enrollment remains functional. Only the "My Grades" widget is replaced by a temporary error state.

### 3. Concurrency Safety

The Course Service manages enrollment slots using **Atomic Mutexes**, preventing race conditions where two students might grab the last seat simultaneously.

---

## How to Run (Docker Method)

**Prerequisites:** Docker Desktop installed.

### 1. Build & Start the Cluster

This spins up all 4 services and creates the isolated bridge network `172.20.0.0/16`.

```bash
docker-compose up --build

```

**Verify:**

1. Open [http://localhost:8080](https://www.google.com/search?q=http://localhost:8080).
2. Login with: `student1` / `pass123`.

### 2. The "Fault Tolerance" Demo

Demonstrate that the application survives a backend node failure.

1. Keep your browser open on the Dashboard.
2. Open a terminal and kill the Grading Service:
```bash
docker stop grade-service

```


3. **Refresh the page.**
4. Note the **Red Error Box** under "My Grades".
5. Note that **Course Enrollment** still works perfectly.

### 3. The "Security" Demo (Hacker Test)

Prove that the API is secured against unauthorized access and role spoofing.

```bash
# Attempt to access grades without a token
curl -i http://localhost:8083/grades?student_id=student1
# Result: 401 Unauthorized

# Attempt to access Faculty data as a Student (Replace <TOKEN> with your browser cookie)
curl -i -H "Authorization: Bearer <STUDENT_TOKEN>" "http://localhost:8083/grades?student_id=faculty1"
# Result: 403 Forbidden

```

---

## Project Structure

```text
distributed-enrollment/
├── docker-compose.yml       # Orchestration & Network Definitions
├── portal/                  # [Node 1] Frontend Gateway & Circuit Breaker Logic
├── auth-service/            # [Node 2] JWT Issuance & Validation
├── course-service/          # [Node 3] Course Catalog & Mutex Logic
└── grade-service/           # [Node 4] Introspection & RBAC Logic

```

## User Accounts (Test Data)

The system is pre-loaded with the following accounts for testing:

| Username | Password | Role | Capabilities |
| --- | --- | --- | --- |
| **student1** | `pass123` | Student | Can enroll, View own grades. |
| **faculty1** | `pass123` | Faculty | Can View all grades, Upload new grades. |
