//go:build go1.25 && !linux && !darwin

package godtestnet

import "os"

// No unverified platform lock fallback. Persistent disk startup is unavailable.
func acquireNodeLease(*os.Root) (*os.File, error) { return nil, ErrPrivate }
