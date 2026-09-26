# WebsiteCore Web Frontend

The Vue 3 single-page application served by the WebsiteCore backend. In production it is compiled into `dist/` and embedded into the Go binary (see `embed.go`, activated by the `embed` build tag).

## Stack

Vue 3 (script setup, TypeScript) · Vite · Naive UI (auto-imported via unplugin) · Pinia · vue-router (hash history) · vue-advanced-chat (private messaging, registered as a custom element) · md-editor-v3 (Markdown long-form posts) · Artplayer (video playback).

## Commands

```sh
npm install
npm run dev        # Vite dev server
npm run build      # production bundle -> dist/ (embedded by `make build-web` from the repo root)
npm run preview    # preview a production build
npm run lint       # ESLint (quality rules)
npm run lint:fix
npm run format     # Biome (formatting only)
npm run check      # Biome check --write
```

Formatting is owned by **Biome** (`biome.json`); code-quality linting by **ESLint** (`eslint.config.js`, flat config: `vue flat/strongly-recommended` + `typescript-eslint recommended`). CI runs `npm run lint` and `npm run build` on Node 22.

## Configuration

Environment files: `.env` (defaults, committed) and `.env.local` (personal overrides, git-ignored).

- `VITE_HOST` — API base URL (`src/utils/request.ts`). Empty means same-origin, which is correct for the embedded production build. For standalone `npm run dev` against a local backend, set `VITE_HOST="http://127.0.0.1:8008"` in `.env.local`.
- `VITE_USE_WEB_PROFILE=true` — fetch feature flags from the backend `WebProfile` at runtime, making the admin UI (`/#/admin/settings`) authoritative over the `VITE_ALLOW_*` / `VITE_DEFAULT_*` build-time values.

## Layout

```text
src/
├── api/          # backend API wrappers
├── components/   # shared components (compose boxes, chat, course widgets, ...)
├── composables/  # composition helpers
├── router/       # hash-mode routes: /, /post, /compose-md, /courses, /topic,
│                 #   /profile, /u, /messages, /collection, /following, /setting,
│                 #   /admin/{settings,users,audit}, /404
├── store/        # Pinia stores
├── utils/        # request.ts (axios instance), helpers
└── views/        # route-level pages
```

## Rules for contributors

- `dist/` is a build artifact — never commit its contents (the `.gitkeep` placeholder must stay).
- UI copy is written in Chinese and must stay appropriate for a community of minors.
- Any layout change must pass the frontend BVT (no overlapping/overflowing content at the standard viewports) — see the root [`CONTRIBUTING.md`](../CONTRIBUTING.md) and the Playwright scripts under `../scripts/`.
