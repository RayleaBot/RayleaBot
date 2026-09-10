package lifecycle

import "testing"

func TestControllerRejectsMissingRequiredDependencies(t *testing.T) {
	if controller, err := NewController(Deps{}); controller != nil || err == nil {
		t.Fatalf("incomplete lifecycle accepted: controller=%v err=%v", controller, err)
	}
}
