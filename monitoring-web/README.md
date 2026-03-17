# Monitoring Web

Lightweight Next.js UI for local backend log monitoring.

## Local setup

```bash
npm install
npm run dev
```

Open `http://localhost:3000`.

## How to use

1. Ensure the backend is running (default: `http://localhost:8088`).
2. In the UI, set **Backend URL** to your backend address.
3. Click **Start** for continuous polling, or **Poll now** for one request.

The frontend fetches `GET /api/logs/events`, which proxies to:

`GET <backend-url>/v1/logs/events?since=<sequence>&limit=<limit>`

Each event card renders full message/error text plus expandable JSON panels for
`metadata`, `payload`, and the full raw event object.
