package runpod

import "testing"

func TestSuggestFallsBackOffline(t *testing.T) {
	c, offline, err := Suggest(nil, 18, "")
	if err != nil {
		t.Fatal(err)
	}
	if !offline {
		t.Fatal("expected offline")
	}
	if c.Pool.MemoryGB < 18 {
		t.Fatalf("pool too small: %+v", c.Pool)
	}
}

func TestOfflineCatalogNonEmpty(t *testing.T) {
	if len(OfflineCatalog()) < 3 {
		t.Fatal("expected offline pools")
	}
}
