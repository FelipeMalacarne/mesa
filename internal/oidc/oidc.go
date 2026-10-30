// Package oidc signs people into Mesa through an OpenID Connect provider
// (Zitadel, Keycloak, Authentik, Google, …): discovery, the authorization
// code flow with PKCE, and a verified ID token. It implements the provider
// port of app/accounts.
package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/felipemalacarne/mesa/internal/app/accounts"
)

type provider struct {
	p        *gooidc.Provider
	verifier *gooidc.IDTokenVerifier
	conf     oauth2.Config
	client   *http.Client
}

// New discovers the provider at c.Issuer. The discovered keys are fetched
// again as the provider rotates them, outside of any one request.
func New(ctx context.Context, c accounts.ProviderConfig) (accounts.Provider, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	// The key set keeps this context for later refreshes: it must outlive
	// the request that happened to discover the provider.
	pctx := gooidc.ClientContext(context.WithoutCancel(ctx), client)
	p, err := gooidc.NewProvider(pctx, c.Issuer)
	if err != nil {
		return nil, err
	}
	return &provider{
		p:        p,
		verifier: p.Verifier(&gooidc.Config{ClientID: c.ClientID}),
		conf: oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			Endpoint:     p.Endpoint(),
			Scopes:       append([]string{gooidc.ScopeOpenID, "email", "profile"}, c.Scopes...),
		},
		client: client,
	}, nil
}

func (p *provider) AuthCodeURL(redirectURL, state, nonce, verifier string) string {
	conf := p.conf
	conf.RedirectURL = redirectURL
	return conf.AuthCodeURL(state, gooidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

func (p *provider) Exchange(ctx context.Context, redirectURL, code, verifier, nonce string) (*accounts.Identity, error) {
	ctx = gooidc.ClientContext(ctx, p.client)
	conf := p.conf
	conf.RedirectURL = redirectURL
	tok, err := conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange the code: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return nil, errors.New("the provider returned no ID token")
	}
	idt, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("verify the ID token: %w", err)
	}
	if idt.Nonce != nonce {
		return nil, errors.New("the ID token answers another sign-in (nonce mismatch)")
	}
	claims := map[string]any{}
	if err := idt.Claims(&claims); err != nil {
		return nil, fmt.Errorf("read the ID token: %w", err)
	}
	// Providers keep some claims out of the ID token (Zitadel: the email,
	// and the roles unless asked to put them in): the userinfo endpoint has
	// them. What the ID token says wins.
	if info, err := p.p.UserInfo(ctx, oauth2.StaticTokenSource(tok)); err == nil && info.Subject != "" {
		if info.Subject != idt.Subject {
			return nil, errors.New("the user info belongs to someone else")
		}
		extra := map[string]any{}
		if err := info.Claims(&extra); err == nil {
			for k, v := range extra {
				if _, ok := claims[k]; !ok {
					claims[k] = v
				}
			}
		}
	}
	str := func(k string) string { v, _ := claims[k].(string); return v }
	name := str("name")
	if name == "" {
		name = str("preferred_username")
	}
	return &accounts.Identity{
		Issuer: idt.Issuer, Subject: idt.Subject, Email: str("email"), Name: name, Claims: claims,
	}, nil
}
