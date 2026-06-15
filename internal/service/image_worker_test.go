package service

import "testing"

func TestKeyExtensionChanged(t *testing.T) {
	if !KeyExtensionChanged("recipe/a.webp", "recipe/a.png") {
		t.Fatal("expected changed")
	}
	if KeyExtensionChanged("recipe/a.jpg", "recipe/a.jpg") {
		t.Fatal("expected same")
	}
}

func TestBuildObjectURLNilConfig(t *testing.T) {
	_, err := BuildObjectURL("recipe/x.png")
	if err == nil {
		t.Fatal("expected error without config")
	}
}
