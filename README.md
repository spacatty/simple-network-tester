# Simple Network Tester

A local load-testing application with a sleek dark dashboard.

## Features

- GET and POST load tests
- Body types: JSON, form urlencoded, multipart/form-data with file uploads
- Custom success status list (example: `200,400`)
- User-Agent configuration:
  - default fake user-agent rotation
  - custom user-agent list rotation
  - fixed user-agent
- Proxy management:
  - import proxy list
  - recheck proxy validity
  - delete inactive proxies
  - global proxy enable/disable
  - proxy failover on proxy/transport errors
- Dynamic live dashboard:
  - latency timeline
  - status distribution
  - run summary cards
  - historical reports + CSV export
- Token authentication:
  - backend requires `AUTH_TOKEN`
  - browser sends it as an `Authorization: Bearer ...` header
  - token is entered in the UI and kept in browser session storage, not bundled into frontend env

## Stack

- Frontend: Next.js (TypeScript, Tailwind, shadcn-style UI primitives, Recharts)
- Backend: Go HTTP API + SQLite

## Project Structure

- `frontend` - dashboard web app
- `backend` - load runner, proxy manager, storage, reporting API

## Run Locally

The recommended way to run the app is with [Task](https://taskfile.dev/). Copy `.env.example` to `.env` and configure shared settings first:

```dotenv
AUTH_TOKEN=use-a-long-random-token
BACKEND_PORT=9090
BACKEND_DATA_DIR=./data
FRONTEND_PORT=4000
NEXT_PUBLIC_API_BASE=http://localhost:9090
```

Development mode:

```bash
task dev
```

Production mode:

```bash
task prod
```

Run checks:

```bash
task test
```

### 1) Backend

```bash
cd backend
export AUTH_TOKEN='use-a-long-random-token'
go run ./cmd/server
```

Backend defaults:

- Port: `8080`
- Data directory: `./data`
- SQLite file: `./data/loadtester.db`
- Auth token: required via `AUTH_TOKEN`

### 2) Frontend

```bash
cd frontend
npm install
npm run dev
```

Frontend defaults:

- URL: `http://localhost:3000`
- API base: `http://localhost:8080` (configurable by env)

## Environment Variables

### Backend

- `PORT` (default `8080`)
- `DATA_DIR` (default `./data`)
- `AUTH_TOKEN` (required)

### Frontend

- `NEXT_PUBLIC_API_BASE` (default `http://localhost:8080`)

Do not put the auth token in frontend environment variables. Enter it in the dashboard token field.

## API Endpoints

- `GET /api/health`
- `POST /api/files/upload`
- `POST /api/proxies/import`
- `GET /api/proxies`
- `POST /api/proxies/recheck`
- `POST /api/proxies/delete-inactive`
- `POST /api/proxies/enabled`
- `POST /api/runs/start`
- `GET /api/runs/stream?runId=...` (SSE)
- `GET /api/reports`
- `GET /api/reports/export?runId=...` (CSV)

All API endpoints except `GET /api/health` require:

```http
Authorization: Bearer <AUTH_TOKEN>
```

## Tests

Backend:

```bash
cd backend
go test ./...
```

Frontend:

```bash
cd frontend
npm run lint
npm run build
```

## Important Note

Run load tests only against applications you own or have explicit authorization to test.