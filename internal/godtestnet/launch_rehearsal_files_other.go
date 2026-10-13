//go:build go1.25 && !linux && !darwin

package godtestnet

import "os"

func openLaunchRoot(string) (*os.Root, error)                { return nil, ErrLaunchRehearsal }
func launchRootMatches(*os.Root, string) bool                { return false }
func readLaunchFile(*os.Root, string, int64) ([]byte, error) { return nil, ErrLaunchRehearsal }
func launchDataDirectory(*os.Root, string) error             { return ErrLaunchRehearsal }
