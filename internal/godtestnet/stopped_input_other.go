//go:build go1.25 && !linux && !darwin

package godtestnet

import "os"

func readStoppedFile(*os.Root, string, int64) (stoppedInput, error) {
	return stoppedInput{}, ErrUpgradeCheck
}
