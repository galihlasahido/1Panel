package v2

import (
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/gin-gonic/gin"
)

// scopeNodes filters a node list to the session user's allowed nodes.
// Superadmin (or no resolvable session — left to other guards) sees
// the full list. Keeps RBAC node visibility consistent with the
// per-request enforcement in middleware.RBACEnforce.
func scopeNodes[T any](c *gin.Context, items []T, nameOf func(T) string) []T {
	su, err := global.SESSION.Get(c)
	if err != nil || su.IsSuper {
		return items
	}
	allowed := func(name string) bool {
		for _, n := range su.Nodes {
			if n == "*" || n == name {
				return true
			}
		}
		return false
	}
	out := make([]T, 0, len(items))
	for _, it := range items {
		if allowed(nameOf(it)) {
			out = append(out, it)
		}
	}
	return out
}
