//go:build compatibility

package openai_compatibility

import "reflect"

func observationsEqual(direct, shell observation) bool {
	return reflect.DeepEqual(direct.Cases, shell.Cases)
}
