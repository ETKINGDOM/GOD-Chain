package godrh

import "path/filepath"

func custodyNonceSeparated(directory string, i CustodyTransactionInputs) bool {
	if directory == "" {
		return true
	}
	physical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return false
	}
	for _, p := range []string{i.CallInputs.ConfigurationPath, i.CallInputs.RequestPath, i.PlanPath, i.TransactionPath} {
		parent, err := filepath.EvalSymlinks(filepath.Dir(p))
		if err != nil || parent == physical {
			return false
		}
	}
	return true
}
