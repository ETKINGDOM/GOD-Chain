//go:build go1.25 && !linux && !darwin

package goddeploy

import "os"

func readHostMetrics(*os.File) (hostMetrics, bool, error) {
	return hostMetrics{}, false, ErrHost
}
