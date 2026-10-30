# ADR 0012: Provisioning an app's database writes to the database and to Git

Status: accepted · 2026-10-29

## Context

Giving a new app a database on a CloudNativePG cluster kept in Git meant four hand edits: a managed role in the `Cluster`, a `Database` resource, a Secret with the role's password, and the app's own Secret with the same password in a URL, encrypted with sops. Mesa could already create the role and the database with SQL, but a database created only in SQL is invisible to the repository, and Argo CD or Flux would never know about it. ADR 0010 first proposed handing the manifests to the person to commit; that keeps Git right but leaves the clunky part to them.

## Decision

A connection declared with a `gitops` table (repository, branch, a folder Mesa owns, the CNPG Cluster) can provision: one form (a name, the app's namespace) does both halves.

1. **Git.** Mesa commits, in one commit, to a Kustomize component it owns next to the cluster's declaration (`internal/cnpg`): the CNPG `Database`, the role as a managed role patched into the `Cluster`, and a sops file with the role's password (for CNPG) and the app's connection Secret `<name>-db` in the app's namespace (`DATABASE_URL`, `uri`, `username`, `password`, `host`, `port`, `dbname`). It encrypts for the age recipients the repository's `.sops.yaml` gives (`internal/sops`), never holding a private key, and writes through the GitHub API (`internal/github`) with `MESA_GITHUB_TOKEN`. Mesa rewrites the component's index (kustomization, roles patch, KSOPS generator) from the folder's listing and never touches a file it did not write.
2. **Database, now.** Then it creates the role (with a generated password nobody sees) and the database, through the same plan, policy and audit as any write: production must be unlocked and named.

The commit lands first: once it is in, the cluster converges on its own (CNPG creates or adopts the role and the database) even if the SQL fails. With `pull_request = true` Mesa opens a pull request instead and runs nothing until it is merged and synced.

## Consequences

- The parent kustomization includes the component once and must leave explicit namespaces alone (a `NamespaceTransformer` with `unsetOnly: true` instead of `namespace:`), because the app's Secret lives in the app's namespace.
- The app's namespace must exist; Mesa does not create namespaces.
- Mesa can add secrets to the repository but not read or change existing ones: rotating a provisioned password, or dropping a database, is a hand edit or a later feature.
- Only admins provision; the commit and the SQL are both in the audit log.
