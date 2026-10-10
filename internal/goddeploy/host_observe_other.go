//go:build go1.25 && !linux

package goddeploy

func openHostObservation(HostObserveOptions) (hostObservationSource, string, error) {
	return nil, "unsupported-runtime", ErrHostObserve
}
