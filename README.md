# urban-octo-umbrella

Single-binary PocketBase + Go backend with a React (Vite) frontend.

## Dev

1. Install dependencies:
   - `pnpm --dir web install`
2. Run backend:
   - `pnpm run dev:server`
3. Run frontend:
   - `pnpm run dev:web`

The Vite dev server proxies `/api` and `/_/` to the backend on `http://localhost:8090`.

## Build (single binary)

1. Build the frontend:
   - `pnpm run build:web`
2. Build the Go binary:
   - `pnpm run build:server`

The Go binary embeds the frontend build output from `server/web/dist`.
