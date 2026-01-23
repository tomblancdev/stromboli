// Package session manages session lifecycle and persistent storage.
//
// # Session IDs
//
// Sessions are identified by UUIDs in the format "sess-<uuid>".
// The prefix makes session IDs easily identifiable in logs and URLs.
//
// # Usage
//
// Generate a new session ID:
//
//	id := session.NewID()  // Returns "sess-abc123..."
//
// Validate a session ID:
//
//	if err := session.Validate(id); err != nil {
//	    // Invalid format
//	}
//
// # Storage
//
// Sessions are stored as directories under the configured sessions path.
// Each session directory contains Claude's conversation state and can be
// resumed or forked for continued conversations.
//
// # Persistence Options
//
// Sessions support two modes:
//   - Persistent: Session data is saved to disk (default)
//   - Ephemeral: Session data is stored in tmpfs and lost on container stop
package session
