package discovery

import (
	"testing"
)

func TestSmitheryConnectorCapabilities(t *testing.T) {
	c := SmitheryConnector{}
	if c.Name() != "Smithery" {
		t.Fatalf("expected Name() to be 'Smithery', got %q", c.Name())
	}
	if !c.Capabilities().RequiresPayment {
		t.Fatal("Smithery is a real, paid source -- RequiresPayment should be true")
	}
}

func TestWebConnectorCapabilities(t *testing.T) {
	c := WebConnector{}
	if c.Name() != "Web search" {
		t.Fatalf("expected Name() to be 'Web search', got %q", c.Name())
	}
	if !c.Capabilities().RequiresPayment {
		t.Fatal("Web search is a real, paid source -- RequiresPayment should be true")
	}
}
