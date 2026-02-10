# Project Structure

```text
.
├── server/               Go app embedding PocketBase + frontend assets
│   ├── main.go           PocketBase bootstrap + SPA static file serving
│   └── web/dist/         Frontend build output embedded in the Go binary
├── web/                  React (Vite) frontend
│   ├── src/              React source
│   ├── public/           Static public assets
│   ├── index.html        Vite entry HTML
│   ├── package.json      Frontend dependencies and scripts
│   ├── tsconfig.json     Frontend TypeScript config
│   ├── tsconfig.node.json Vite config TS support
│   └── vite.config.ts    Vite config (proxy + build output)
├── go.mod                Go module definition
├── package.json          Root scripts for dev/build
├── README.md             Project usage docs
└── .gitignore            Repo ignore rules
```
