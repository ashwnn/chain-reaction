package guardrails

import (
	"context"
	"fmt"

	"golang.org/x/time/rate"
)

type Enforcer struct {
	allowedNamespaces map[string]struct{}
	limiter           *rate.Limiter
}

func New(allowList []string, qps float32, burst int) *Enforcer {
	set := make(map[string]struct{}, len(allowList))
	for _, ns := range allowList {
		set[ns] = struct{}{}
	}

	r := rate.Limit(qps)
	if qps <= 0 {
		r = rate.Limit(10)
	}
	b := burst
	if b <= 0 {
		b = 20
	}

	return &Enforcer{
		allowedNamespaces: set,
		limiter:           rate.NewLimiter(r, b),
	}
}

func (e *Enforcer) Acquire(ctx context.Context) error {
	return e.limiter.Wait(ctx)
}

func (e *Enforcer) CheckNamespace(namespace string) error {
	if len(e.allowedNamespaces) == 0 {
		return nil
	}
	if _, ok := e.allowedNamespaces[namespace]; ok {
		return nil
	}
	return fmt.Errorf("namespace %q is outside allow-list", namespace)
}
