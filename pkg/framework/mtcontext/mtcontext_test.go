package mtcontext

import (
	"context"
	"testing"
)

func TestTenantUID(t *testing.T) {
	t.Run("ContextWithTenantUID and TenantUIDFromContext", func(t *testing.T) {
		ctx := context.Background()
		uid := "test-uid"
		ctx = ContextWithTenantUID(ctx, uid)

		got := TenantUIDFromContext(ctx)
		want := "tenant-uid:test-uid"
		if got != want {
			t.Errorf("TenantUIDFromContext() = %v, want %v", got, want)
		}
	})

	t.Run("TenantUIDFromContext with empty context", func(t *testing.T) {
		if got := TenantUIDFromContext(context.Background()); got != nil {
			t.Errorf("TenantUIDFromContext(context.Background()) = %v, want nil", got)
		}
	})

	t.Run("ContextWithTenantUID with empty string UID", func(t *testing.T) {
		ctx := context.Background()
		uid := ""
		ctx = ContextWithTenantUID(ctx, uid)

		got := TenantUIDFromContext(ctx)
		want := "tenant-uid:"
		if got != want {
			t.Errorf("TenantUIDFromContext() = %v, want %v", got, want)
		}
	})
}
