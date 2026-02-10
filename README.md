# urban-octo-umbrella

Single-binary PocketBase + Go backend with a React (Vite) frontend.

## Dev

1. Install dependencies:
   - `npm --prefix web install`
2. Run backend:
   - `npm run dev:server` (uses `-tags=dev` to skip embedded assets)
3. Run frontend:
   - `npm run dev:web`

The Vite dev server proxies `/api` and `/_/` to the backend on `http://localhost:8090`.

## Build (single binary)

1. Build the frontend:
   - `npm run build:web`
2. Build the Go binary:
   - `npm run build:server`

The Go binary embeds the frontend build output from `server/web/dist`.
