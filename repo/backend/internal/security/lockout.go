package security

import "time"

const (
	MaxFailedAttempts = 5
	LockoutDuration   = 15 * time.Minute
)

func IsLocked(failedAttempts int, lockedUntil *time.Time, now time.Time) bool {
	if lockedUntil == nil {
		return failedAttempts >= MaxFailedAttempts
	}
	return now.Before(*lockedUntil)
}

func NextLockout(now time.Time) time.Time {
	return now.Add(LockoutDuration)
}
