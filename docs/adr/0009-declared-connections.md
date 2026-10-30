# ADR 0009: A deployment declares its connections in a file

Status: accepted · 2026-10-29

## Context

In server mode, connections were added through the web app or the API and lived only in Mesa's store. A deployment kept in Git (Kubernetes manifests, a compose file) could not say which databases its Mesa reaches: a new store started empty, and the list had to be clicked back in. The project folder (ADR 0005) answers this for one application's environments, not for a server that looks after many unrelated databases.

## Decision

`MESA_CONNECTIONS` names a TOML file with one `[connections.<name>]` table per connection: `engine`, `environment`, `host`, `port`, `database`, `user`, `password`, `tls`, `read_only`. `store/declared` reads it at start and layers it over the store and the vault the way a project does:

- declared connections are listed with the stored ones, under IDs derived from their names, so history, layouts and audit follow them across restarts and replicas;
- they are read-only through every surface, and `definedIn` names the file;
- passwords are only references (`env:NAME`, `keychain:service/account`), resolved when a session opens;
- a file that does not validate (unknown key, literal password, missing host) stops Mesa from starting, because it belongs to the deployment.

The Helm chart renders `values.connections` into a ConfigMap, mounts it, and rolls the pods when it changes. It passes `config.existingSecret` whole as environment, so the referenced passwords and a PostgreSQL store URL can sit in one Secret.

## Consequences

- Git owns the declared list; people still add their own connections next to it.
- A change needs a restart (the chart's checksum annotation triggers it); there is no reload.
- This is the first source of connections besides the store. Kubernetes discovery (ADR 0010) is meant to be the next one, through the same overlay.
