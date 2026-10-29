# ADR 0011: People sign in through an identity provider, which gives their role

Status: accepted · 2026-10-29

## Context

`mesa-server` had no sign-in: anyone who reached it could do everything, and the audit log named the surface ("web"), not the person. Putting it behind a VPN or a proxy's password made it usable but not accountable. Organisations that run Mesa already run an identity provider (Zitadel, Keycloak, Authentik, Google): they decide there who exists, who left, and who may do what.

## Decision

- **Sign-in is OpenID Connect, and only that.** Mesa stores no passwords and manages no members. With `MESA_OIDC_ISSUER` and `MESA_OIDC_CLIENT_ID` (and usually `MESA_OIDC_CLIENT_SECRET`), the browser goes to `/auth/login`, through the provider with the authorization code flow and PKCE, and back to `/auth/callback`; the ID token is verified (go-oidc) and the userinfo claims merged in.
- **The provider gives the role**: `admin`, `editor` or `viewer`, read from Zitadel's project roles claims, or from `roles` or `groups` (names may carry a `mesa` prefix), the highest one found. Nobody without one gets in, unless `MESA_OIDC_DEFAULT_ROLE` says otherwise.
- **Roles map to three permissions** in `core/account`: viewer reads; editor also writes (applies changes, unlocks, cancels sessions, saves queries); admin also manages connections and provisions databases. Use cases check them with `account.Require`, so every surface is held to the same rule; a context without a principal is refused.
- **Sessions are short**: an HttpOnly, SameSite=Lax cookie for 12 hours, so a role taken away in the provider ends here within the day; signing in again is one redirect while the provider's own session lasts. State-changing requests carrying the cookie from another site are refused.
- **Agents use API tokens**: a person makes one in Settings and sends it as a bearer token to `/mcp` or `/api`; it acts with the role of their last sign-in. Only the SHA-256 of sessions and tokens is stored.
- **Local surfaces have no sign-in.** The desktop app, the CLI, `mesa-mcp` on stdio, and a server without an issuer run as the local principal, who may do everything; the server says so in its log.

## Consequences

- The audit log names the person (their email) for every write from the web and from agents.
- Members, invitations, password resets and two-factor sign-in are the provider's job.
- A role change takes effect at the next sign-in, at most 12 hours later; an API token follows the role of its owner's last sign-in.
