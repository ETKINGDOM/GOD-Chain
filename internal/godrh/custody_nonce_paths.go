package godrh

import "path/filepath"

func custodyNonceSeparated(directory string, i CustodyTransactionInputs) bool {
	if directory == "" {
		return true
	}
	for _, p := range []string{i.CallInputs.ConfigurationPath, i.CallInputs.RequestPath, i.PlanPath, i.TransactionPath} {
		if filepath.Dir(p) == directory {
			return false
		}
	}
	return true
}
