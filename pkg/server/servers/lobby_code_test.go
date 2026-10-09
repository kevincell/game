package servers

import (
	"regexp"
	"testing"
)

func TestServerManagerReservesReadableUniqueLobbyCodes(t *testing.T) {
	valid := regexp.MustCompile(`^[ABCDEFGHJKMNPQRSTUVWXYZ23456789]{5}$`)
	manager := &ServerManager{identities: make(map[string]struct{})}
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		code := manager.nextIdentity()
		if !valid.MatchString(code) {
			t.Fatalf("generated invalid lobby code %q", code)
		}
		if _, exists := seen[code]; exists {
			t.Fatalf("generated duplicate code %q", code)
		}
		seen[code] = struct{}{}
	}
}
