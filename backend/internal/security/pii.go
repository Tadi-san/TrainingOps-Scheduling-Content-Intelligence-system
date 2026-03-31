package security

import "strings"

func MaskEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return Mask(email)
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= 2 {
		return strings.Repeat("*", len(local)) + "@" + domain
	}
	return local[:2] + strings.Repeat("*", len(local)-2) + "@" + domain
}
