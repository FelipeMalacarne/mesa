package secret

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

type memBlobs map[uuid.UUID][]byte

func (m memBlobs) Get(_ context.Context, id uuid.UUID) ([]byte, error) {
	b, ok := m[id]
	if !ok {
		return nil, errs.New(errs.NotFound, "no secret")
	}
	return b, nil
}
func (m memBlobs) Set(_ context.Context, id uuid.UUID, b []byte) error { m[id] = b; return nil }
func (m memBlobs) Delete(_ context.Context, id uuid.UUID) error        { delete(m, id); return nil }

func TestRoundTrip(t *testing.T) {
	blobs := memBlobs{}
	v, err := NewLocal("passphrase", blobs)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if err := v.Set(context.Background(), id, "hunter2"); err != nil {
		t.Fatal(err)
	}
	if string(blobs[id]) == "hunter2" {
		t.Fatal("stored in plaintext")
	}
	got, err := v.Get(context.Background(), id)
	if err != nil || got != "hunter2" {
		t.Fatalf("got %q %v", got, err)
	}
	other, _ := NewLocal("different", blobs)
	if _, err := other.Get(context.Background(), id); !errs.Is(err, errs.Forbidden) {
		t.Fatalf("wrong key should fail: %v", err)
	}
}

func TestEnvReference(t *testing.T) {
	t.Setenv("MESA_TEST_SECRET", "from-env")
	blobs := memBlobs{}
	v, _ := NewLocal("k", blobs)
	id := uuid.New()
	_ = v.Set(context.Background(), id, "env:MESA_TEST_SECRET")
	got, err := v.Get(context.Background(), id)
	if err != nil || got != "from-env" {
		t.Fatalf("got %q %v", got, err)
	}
	_ = v.Set(context.Background(), id, "env:MESA_MISSING_SECRET_X")
	if _, err := v.Get(context.Background(), id); !errs.Is(err, errs.Unavailable) {
		t.Fatalf("missing env: %v", err)
	}
}
