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
npm run i18n:check # locale pack validation (missing / unused / zh-CN-en parity)
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
├── locales/      # i18n: index.ts (setup) + <locale>/<namespace>.json packs
├── router/       # hash-mode routes: /, /post, /compose-md, /courses, /topic,
│                 #   /profile, /u, /messages, /collection, /following, /setting,
│                 #   /admin/{settings,users,audit}, /404
├── store/        # Pinia stores
├── utils/        # request.ts (axios instance), helpers
└── views/        # route-level pages
```

## Internationalization (i18n)

Built on **vue-i18n v11** (composition mode) with `@intlify/unplugin-vue-i18n` precompiling the JSON packs at build time.

- **Locales**: `zh-CN` (source language) and `en`. Language picker: the globe button in the bottom-left user area of the sidebar (available both logged-in and as a guest). The choice persists in `localStorage` (`PAOPAO_LOCALE`); first visits follow the browser language.
- **Pack layout**: `src/locales/<locale>/<namespace>.json` — the file name is the namespace, so a key reads `post.action.delete`. Namespaces: `common`, `nav`, `sidebar`, `auth`, `post`, `comment`, `compose`, `message`, `user`, `setting`, `course`, `admin{Audit,Users,Settings}`, `errors`.
- **Translation platforms**: the packs are plain nested JSON, ready to import into Crowdin / Weblate / Tolgee so the community can maintain additional languages. Add a new locale by creating `src/locales/<lang>/` with the same file/key structure and registering it in `SUPPORTED_LOCALES` + `localeOptions` (`src/locales/index.ts`).
- **Backend errors**: the Go backend still returns Chinese messages. The frontend maps error codes to `errors.codes.E<code>` keys (`src/locales/errorCodes.ts`, applied in `src/utils/request.ts`); unmapped codes fall back to the server message.
- **Conventions**: use `const { t } = useI18n()` in components and `i18n.global.t()` in plain `.ts` modules (call it at run time, never freeze the result in a module-level constant). Static label arrays/maps in `setup` must be `computed` so they react to language switches. Named interpolation only (`{count}`); literal `@`/`|`/`{`/`}` in copy must be escaped (`{'@'}`, `{'|'}`, ...).
- **Validation**: `npm run i18n:check` fails on missing keys or zh-CN/en structure drift and warns on unused keys.

## Rules for contributors

- `dist/` is a build artifact — never commit its contents (the `.gitkeep` placeholder must stay).
- Never hardcode UI copy in components — add keys to `src/locales/zh-CN/<namespace>.json` **and** `src/locales/en/<namespace>.json`, then render with `t()`. Copy must stay appropriate for a community of minors. Run `npm run i18n:check` before committing.
- Any layout change must pass the frontend BVT (no overlapping/overflowing content at the standard viewports) — see the root [`CONTRIBUTING.md`](../CONTRIBUTING.md) and the Playwright scripts under `../scripts/`.
