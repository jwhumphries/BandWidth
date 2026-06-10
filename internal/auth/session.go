package auth

import "time"

// SessionCookieName is the HTTP cookie carrying the raw session token.
const SessionCookieName = "bandwidth_session"

// SessionDuration is how long a session stays valid.
const SessionDuration = 30 * 24 * time.Hour
