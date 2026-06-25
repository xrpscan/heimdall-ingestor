package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistry_CloseOrder(t *testing.T) {
	t.Parallel()

	var order []string
	reg := New(nil)

	for _, name := range []string{"a", "b", "c"} {
		reg.RegisterWithFunc(name, func(ctx context.Context) error {
			order = append(order, name)
			return nil
		})
	}

	reg.MustCloseAll()
	require.Equal(t, []string{"c", "b", "a"}, order)
}

func TestRegistry_RegisterWithFunc(t *testing.T) {
	t.Parallel()

	called := false
	reg := New(nil)

	reg.RegisterWithFunc("svc", func(ctx context.Context) error {
		called = true
		return nil
	})

	reg.MustCloseAll()
	require.True(t, called)
}

func TestRegistry_DuplicateNamePanics(t *testing.T) {
	t.Parallel()

	reg := New(nil)
	reg.RegisterWithFunc("svc", func(ctx context.Context) error { return nil })

	require.Panics(t, func() {
		reg.RegisterWithFunc("svc", func(ctx context.Context) error { return nil })
	})
}

func TestRegistry_MustCloseAllIdempotent(t *testing.T) {
	t.Parallel()

	callCount := 0
	reg := New(nil)

	reg.RegisterWithFunc("svc", func(ctx context.Context) error {
		callCount++
		return nil
	})

	reg.MustCloseAll()
	reg.MustCloseAll()

	require.Equal(t, 1, callCount)
}

func TestRegistry_CloseErrorPanics(t *testing.T) {
	t.Parallel()

	reg := New(nil)

	reg.RegisterWithFunc("bad", func(ctx context.Context) error {
		return errors.New("shutdown failed")
	})

	require.PanicsWithValue(t,
		"failed to close all services gracefully: failed to close bad gracefully: shutdown failed",
		func() { reg.MustCloseAll() },
	)
}

func TestRegistry_CloseErrorStillClosesOthers(t *testing.T) {
	t.Parallel()

	var closed []string
	reg := New(nil)

	reg.RegisterWithFunc("a", func(ctx context.Context) error {
		closed = append(closed, "a")
		return nil
	})
	reg.RegisterWithFunc("b", func(ctx context.Context) error {
		closed = append(closed, "b")
		return errors.New("b failed")
	})
	reg.RegisterWithFunc("c", func(ctx context.Context) error {
		closed = append(closed, "c")
		return nil
	})

	require.Panics(t, func() { reg.MustCloseAll() })
	require.Equal(t, []string{"c", "b", "a"}, closed)
}

func TestRegistry_NilLogger(t *testing.T) {
	t.Parallel()

	reg := New(nil)
	reg.RegisterWithFunc("svc", func(ctx context.Context) error { return nil })

	require.NotPanics(t, func() { reg.MustCloseAll() })
}

func TestRegistry_EmptyCloseAll(t *testing.T) {
	t.Parallel()

	reg := New(nil)
	require.NotPanics(t, func() { reg.MustCloseAll() })
}
