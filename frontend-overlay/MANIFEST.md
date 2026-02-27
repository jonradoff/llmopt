# Frontend Overlay Manifest

Files in this directory are copied on top of the LastSaaS frontend source
before building. They replace or add to the base LastSaaS UI.

## Replaced files

| File | Purpose |
|------|---------|
| `src/App.tsx` | Routes: auth at root (incl. MFA, magic link), app pages under `/last/`, dashboard redirects to `/` |
| `src/components/Layout.tsx` | Simplified sidebar: Back to App, Team, Plan, Settings |
| `src/pages/app/DashboardPage.tsx` | Redirects to `/` (main llmopt app) |

## Notes

- Auth pages (`/login`, `/signup`, `/auth/mfa`, `/auth/magic-link`, etc.) are served at root level
- Protected app routes live under `/last/` basename
- After login, users are redirected to `/` (the main LLM Optimizer SPA)
- The admin panel is at `/last` (root tenant admins only)
- Uses lazy loading for auth and admin pages (matches upstream LastSaaS pattern)
- Includes ThemeProvider, ErrorBoundary, and Toaster from upstream
- Activity page at `/last/activity`, onboarding at `/last/onboarding`
