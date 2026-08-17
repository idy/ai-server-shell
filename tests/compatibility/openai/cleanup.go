//go:build compatibility

package openai_compatibility

import "fmt"

func validateEvidence(profile string, observation observation) error {
	definition, ok := liveCases[profile]
	if !ok || len(observation.Cases) != 1 {
		return fmt.Errorf("profile %q produced %d cases", profile, len(observation.Cases))
	}
	result := observation.Cases[0]
	if result.Name != definition.Name || result.Outcome != "PASS" || len(result.Observation) == 0 {
		return fmt.Errorf("profile %q produced incomplete evidence for %q", profile, result.Name)
	}
	if result.Cleanup.Required != definition.CleanupRequired {
		return fmt.Errorf("case %q cleanup requirement changed", result.Name)
	}
	wantCleanup := "not_required"
	if definition.CleanupRequired {
		wantCleanup = "verified"
	}
	if result.Cleanup.Outcome != wantCleanup {
		return fmt.Errorf("case %q cleanup = %q, want %q", result.Name, result.Cleanup.Outcome, wantCleanup)
	}
	return nil
}
