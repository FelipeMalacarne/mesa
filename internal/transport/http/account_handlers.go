package http

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/app/accounts"
	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
	"github.com/felipemalacarne/mesa/internal/transport/http/gen"
)

func toUser(u account.User) gen.User {
	return gen.User{
		Id: u.ID, Email: u.Email, Name: u.Name, Role: gen.MemberRole(u.Role),
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt, LastSeenAt: u.LastSeenAt,
	}
}

func toToken(t store.Token) gen.ApiToken {
	return gen.ApiToken{Id: t.ID, Name: t.Name, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt}
}

// accountsOf returns the accounts service, or an error in local mode.
func (h *handlers) accountsOf() (*accounts.Service, error) {
	if h.app.Accounts == nil {
		return nil, errs.New(errs.Unsupported, "this Mesa has no sign-in (no identity provider)")
	}
	return h.app.Accounts, nil
}

func (h *handlers) GetAuthState(ctx context.Context, _ gen.GetAuthStateRequestObject) (gen.GetAuthStateResponseObject, error) {
	if h.app.Accounts == nil {
		return gen.GetAuthState200JSONResponse{Mode: gen.AuthStateModeLocal}, nil
	}
	out := gen.AuthState{Mode: gen.AuthStateModeOidc, Provider: ptr(h.app.Accounts.Label())}
	if p, ok := account.From(ctx); ok && p.User != nil {
		out.User = ptr(toUser(*p.User))
	}
	return gen.GetAuthState200JSONResponse(out), nil
}

func (h *handlers) SignOut(ctx context.Context, _ gen.SignOutRequestObject) (gen.SignOutResponseObject, error) {
	in := info(ctx)
	if h.app.Accounts != nil && in.credential != "" {
		if err := h.app.Accounts.SignOut(ctx, in.credential); err != nil {
			return nil, err
		}
	}
	cookie := sessionCookieHeader(in, "", time.Time{})
	return gen.SignOut204Response{Headers: gen.SignOut204ResponseHeaders{SetCookie: &cookie}}, nil
}

func (h *handlers) GetMe(ctx context.Context, _ gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	acc, err := h.accountsOf()
	if err != nil {
		return nil, err
	}
	me, err := acc.Me(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetMe200JSONResponse(toUser(*me)), nil
}

func (h *handlers) ListTokens(ctx context.Context, _ gen.ListTokensRequestObject) (gen.ListTokensResponseObject, error) {
	acc, err := h.accountsOf()
	if err != nil {
		return nil, err
	}
	list, err := acc.Tokens(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListTokens200JSONResponse, 0, len(list))
	for _, t := range list {
		out = append(out, toToken(t))
	}
	return out, nil
}

func (h *handlers) CreateToken(ctx context.Context, r gen.CreateTokenRequestObject) (gen.CreateTokenResponseObject, error) {
	acc, err := h.accountsOf()
	if err != nil {
		return nil, err
	}
	days := deref(r.Body.ExpiresInDays, 0)
	if days < 0 || days > 3650 {
		return nil, errs.New(errs.Invalid, "the expiry is between 0 (never) and 3650 days")
	}
	tok, err := acc.CreateToken(ctx, r.Body.Name, time.Duration(days)*24*time.Hour)
	if err != nil {
		return nil, err
	}
	return gen.CreateToken201JSONResponse{Token: toToken(tok.Token), Secret: tok.Secret}, nil
}

func (h *handlers) RevokeToken(ctx context.Context, r gen.RevokeTokenRequestObject) (gen.RevokeTokenResponseObject, error) {
	acc, err := h.accountsOf()
	if err != nil {
		return nil, err
	}
	if err := acc.RevokeToken(ctx, r.TokenId); err != nil {
		return nil, err
	}
	return gen.RevokeToken204Response{}, nil
}

func (h *handlers) ListPeople(ctx context.Context, _ gen.ListPeopleRequestObject) (gen.ListPeopleResponseObject, error) {
	acc, err := h.accountsOf()
	if err != nil {
		return nil, err
	}
	list, err := acc.People(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListPeople200JSONResponse, 0, len(list))
	for _, u := range list {
		out = append(out, toUser(u))
	}
	return out, nil
}
