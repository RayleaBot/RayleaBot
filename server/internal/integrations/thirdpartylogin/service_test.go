package thirdpartylogin

import (
	"testing"
)

func TestCompatServiceCreation(t *testing.T) {
	t.Parallel()

	service := NewService(nil, nil)
	if service == nil {
		t.Fatal("NewService returned nil")
	}

	validator := NewAccountValidator(nil, nil)
	if validator == nil {
		t.Fatal("NewAccountValidator returned nil")
	}
}

func TestCompatServiceWithOptions(t *testing.T) {
	t.Parallel()

	service := NewServiceWithOptions(Options{})
	if service == nil {
		t.Fatal("NewServiceWithOptions returned nil")
	}
}
