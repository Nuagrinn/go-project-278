### Hexlet tests and linter status:
[![Actions Status](https://github.com/Nuagrinn/go-project-278/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/Nuagrinn/go-project-278/actions)
[![CI](https://github.com/Nuagrinn/go-project-278/actions/workflows/ci.yml/badge.svg)](https://github.com/Nuagrinn/go-project-278/actions/workflows/ci.yml)

## Deployment

Render URL: add after deployment.

The application is prepared for Docker deployment on Render. Configure these environment variables in the Render dashboard:

```bash
PORT=8080
BASE_URL=<public service url>
DATABASE_URL=<postgres connection string>
SENTRY_DSN=<sentry project dsn>
```

To send a test error to Sentry, temporarily set `ENABLE_SENTRY_TEST_ENDPOINT=true`, deploy the service, and open `/debug/sentry`. Disable the variable after confirming the event in Sentry.

## UI

Run the API and frontend together locally:

```bash
npm run dev
```

The frontend is available at `http://localhost:5173/`. The API runs on `http://localhost:8080/`.

For local browser requests from the frontend, the API allows this origin by default:

```bash
CORS_ALLOWED_ORIGINS=http://localhost:5173
```

In Docker/Render, Caddy serves the frontend and proxies API requests to the Go app.
