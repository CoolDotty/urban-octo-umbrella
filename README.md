# urban-octo-umbrella

## GitHub Auth Setup

The backend uses GitHub OAuth. Configure the environment variables below in `backend/.env`.

Required:
- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- `SESSION_SECRET`

Optional:
- `GITHUB_CALLBACK_URL` (required in production)
- `NO_AUTH=true` (dev-only escape hatch to skip auth)
- `FRONTEND_DEV_URL` (dev-only override for Vite URL, default `http://localhost:5173`)

### Create a GitHub OAuth App

1. Go to GitHub → Settings → Developer settings → OAuth Apps.
2. Create a new OAuth App.
3. Set the Authorization callback URL:
   - Dev: `http://localhost:3000/auth/github/callback`
   - Prod: your backend URL, then set `GITHUB_CALLBACK_URL`

### Example `backend/.env`

```
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret
SESSION_SECRET=some_long_random_string
# GITHUB_CALLBACK_URL=https://your-domain.com/auth/github/callback
# NO_AUTH=true
# FRONTEND_DEV_URL=http://localhost:5173
```

## Frontend Hosting

- Dev: Express reverse-proxies non-API routes to the Vite dev server (so the browser stays on port 3000).
- Prod: Express serves `frontend/dist` and falls back to `index.html` for SPA routes.

Build before running prod:
```
pnpm --filter frontend build
pnpm --filter backend start
```
