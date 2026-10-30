# ADR 0010: Kubernetes and CloudNativePG as the platform Mesa manages

Status: proposed · 2026-10-29 · parts decided in ADR 0011 (sign-in) and ADR 0012 (provisioning)

## Context

Mesa runs as a service inside Kubernetes clusters where PostgreSQL is run by CloudNativePG (CNPG). There, much of what "managing a database" means belongs to the operator, not to SQL: instances and which one is primary, failover and switchover, roles and databases declared in `Cluster.spec.managed.roles` and `Database` resources, WAL archiving and base backups (the Barman Cloud plugin), point-in-time recovery, hibernation. A SQL client that sees only the catalog shows none of it. Worse, a role made with `CREATE ROLE` drifts from the Git repository that declares the cluster.

Mesa is also asked to reach clusters on other networks: a home cluster and a cloud one, joined only by a tailnet.

## Proposal

Mesa learns the platform a database runs on, behind a port the way it learns engines, and stays usable without it.

1. **Kubernetes as a source of connections.** In a cluster, `mesa-server` reads CloudNativePG `Cluster` resources through its ServiceAccount, with a small client over the API's REST endpoints rather than client-go. A cluster opted in with annotations (environment, the Secret holding Mesa's credentials) becomes a connection to `<cluster>-rw.<namespace>.svc` with TLS verified against the cluster's own CA (`<cluster>-ca`). The password is a `k8s:<namespace>/<secret>#password` reference resolved when a session opens, never stored. It joins the declared file (ADR 0009) as a read-only source through the same overlay.
2. **A cluster panel.** Beside Monitor: instances, primary, replication lag, phase and conditions, volume usage, image; and backups: whether WAL archiving works, the last base backup, the recovery window (`firstRecoverabilityPoint`, `lastSuccessfulBackup`), `Backup` and `ScheduledBackup` objects. Read-only to begin with.
3. **Declarations respected.** Roles and databases declared in the `Cluster` and in `Database` resources are marked as managed by CNPG in the tree and in Roles, and policy refuses to alter or drop them through SQL, with a hint to change the manifest. New ones Mesa creates in both places at once: decided in ADR 0012.
4. **Operations as changes.** An on-demand backup (a new `Backup`), a restart or switchover (what `kubectl cnpg` does), and a point-in-time recovery wizard that writes a recovery `Cluster` manifest. Each is a new kind of change in `app/changes`: planned as the object or patch Mesa will send, checked by policy, confirmed by name in production, applied and audited, the same path as SQL.
5. **Remote clusters through an agent.** Today a remote cluster can publish its `-rw` service on the tailnet (a CNPG additional managed service of type `LoadBalancer`) and be a declared connection, without the panel. Next, `mesa-agent` (its own binary under `cmd/agent`, per ADR 0002) runs in each remote cluster with its own ServiceAccount and dials out to `mesa-server` over one authenticated, multiplexed connection. It reports the inventory and carries both the Kubernetes calls and the PostgreSQL streams, so a remote cluster needs no inbound exposure, no shared network and no credentials copied to the control plane. Agents enrol with a one-time token and are pinned by key afterwards.
6. **A login before any of this writes.** Decided in ADR 0011: sign-in through the identity provider, which gives the role (viewer, editor, admin); the signed-in person is the audit actor.

Suggested order for what remains: 1 and 2 (read-only, in-cluster), 3, 4, 5.

## Alternatives

- **Kubeconfigs for remote API servers** (for example through Tailscale's API server proxy): fewer parts in Mesa, but every API server must be reachable from the control plane and long-lived tokens live there.
- **Only SQL**: keeps Mesa engine-only, but leaves backups, failover and declarations invisible, which is what managing CNPG mostly is.

## Consequences

- `app` never imports Kubernetes code. Use cases ask the platform port what it supports (backups, switchover), as they ask an engine's `Capabilities()`.
- The chart gains a ServiceAccount and RBAC: read clusters, backups, scheduled backups, databases and poolers, read the Secrets named in annotations; create backups and patch clusters only when operations are turned on.
- Tests run against an `httptest` API server serving recorded CNPG objects; an end-to-end job on kind with the operator comes later.
