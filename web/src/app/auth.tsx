/**
 * Sign-in. A Mesa server with an identity provider (Zitadel, Keycloak, …) shows the sign-in
 * page until the browser has a session; the provider decides who gets in and with which role
 * (viewer, editor, admin). The desktop app and a server without a provider have no sign-in:
 * everything is allowed there. Any request answered with "unauthenticated" brings the page back
 * (see providers.tsx).
 */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { LogIn } from "lucide-react";

import { getGetAuthStateQueryKey, signOut, useGetAuthState } from "@/lib/api/auth/auth";
import type { MemberRole } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { EmptyState } from "@/ui/controls";

const rank: Record<MemberRole, number> = { viewer: 1, editor: 2, admin: 3 };

/** The auth state, cached: it changes only on sign-in and sign-out. */
export function useAuth() {
  return useGetAuthState({ query: { staleTime: 5 * 60_000, retry: 1 } });
}

/** Whether the person may do what `role` may: always in local mode. */
export function useCan(role: MemberRole): boolean {
  const auth = useAuth();
  const s = auth.data;
  if (!s) return false;
  if (s.mode === "local") return true;
  return !!s.user && rank[s.user.role] >= rank[role];
}

export function useSignOut() {
  const qc = useQueryClient();
  return async () => {
    try {
      await signOut();
    } finally {
      qc.clear();
      await qc.invalidateQueries({ queryKey: getGetAuthStateQueryKey() });
    }
  };
}

export function AuthGate({ children }: { children: React.ReactNode }) {
  const auth = useAuth();
  if (auth.isPending) return <div className="m-app" style={{ height: "100%" }} />;
  if (auth.isError) {
    return (
      <Centered>
        <EmptyState title="Mesa is not answering" description={errorMessage(auth.error)} />
      </Centered>
    );
  }
  if (auth.data.mode === "oidc" && !auth.data.user) return <SignIn provider={auth.data.provider ?? "single sign-on"} />;
  return <>{children}</>;
}

function SignIn({ provider }: { provider: string }) {
  // A failed sign-in comes back as /?signin_error=…; show it once.
  const [error] = React.useState(() => new URLSearchParams(window.location.search).get("signin_error"));
  React.useEffect(() => {
    if (error) window.history.replaceState(null, "", window.location.pathname);
  }, [error]);
  return (
    <Centered>
      <EmptyState
        icon={<LogIn className="m-icon" aria-hidden="true" />}
        title="Sign in to Mesa"
        description={error ? <span className="m-help m-help--error">{error}</span> : `Mesa uses ${provider} to know who you are and what you may do.`}
        actions={
          <a className="m-btn m-btn--primary" href="/auth/login">
            Sign in with {provider}
          </a>
        }
      />
    </Centered>
  );
}

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <div className="m-app" style={{ height: "100%", display: "grid", placeItems: "center", padding: 24 }}>
      {children}
    </div>
  );
}
