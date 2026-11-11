package loginutils

// AdminPasswordForRPC marks the raw-password protocol at the API boundary.
// Do not trim or hash here: spaces and a literal "raw-v1:" can be password data.
func AdminPasswordForRPC(password string) string { return "raw-v1:" + password }
