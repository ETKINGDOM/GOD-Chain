//go:build go1.25 && !linux && !darwin

package goddeploy

import "os"

func privateInfo(os.FileInfo) bool                          { return false }
func openPrivateRegular(*os.Root, string) (*os.File, error) { return nil, ErrPackage }
