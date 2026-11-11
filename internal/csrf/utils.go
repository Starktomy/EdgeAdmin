package csrf

// Generate creates a form token bound to the requesting browser session.
func Generate(sessionID string) (string, error) {
	return sharedTokenManager.Generate(sessionID)
}

// Validate atomically consumes a token belonging to this browser session.
func Validate(token string, sessionID string) bool {
	return sharedTokenManager.Consume(token, sessionID)
}
