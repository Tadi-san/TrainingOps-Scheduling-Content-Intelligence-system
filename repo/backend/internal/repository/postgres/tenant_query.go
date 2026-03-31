package postgres

import (
	"fmt"
	"strings"
)

func ScopeTenant(baseQuery, tenantColumn string) string {
	trimmed := strings.TrimSpace(baseQuery)
	if strings.Contains(strings.ToLower(trimmed), " where ") {
		return fmt.Sprintf("%s AND %s = $1", trimmed, tenantColumn)
	}
	return fmt.Sprintf("%s WHERE %s = $1", trimmed, tenantColumn)
}
