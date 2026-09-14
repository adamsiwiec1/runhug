package runpod

import "testing"

func ptr(s string) *string { return &s }

func TestPickCheapestFitting(t *testing.T) {
	gpus := []GPU{
		{ID: "NVIDIA GeForce RTX 4090", Name: "RTX 4090", Pool: ptr("ADA_24"), Memory: 24, Availability: "HIGH", Price: Price{Serverless: 1.10}},
		{ID: "NVIDIA A100 80GB PCIe", Name: "A100 PCIe", Pool: ptr("AMPERE_80"), Memory: 80, Availability: "LOW", Price: Price{Serverless: 2.72}},
		{ID: "out of stock 16GB", Name: "tiny", Pool: ptr("AMPERE_16"), Memory: 16, Availability: "NONE", Price: Price{Serverless: 0.40}},
	}
	c, err := Pick(gpus, 18, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Pool.ID != "ADA_24" || c.GPUCount != 1 {
		t.Fatalf("got %+v", c)
	}
}

func TestPickForcedPool(t *testing.T) {
	gpus := []GPU{
		{ID: "4090", Name: "4090", Pool: ptr("ADA_24"), Memory: 24, Availability: "HIGH", Price: Price{Serverless: 1.10}},
		{ID: "A100", Name: "A100", Pool: ptr("AMPERE_80"), Memory: 80, Availability: "HIGH", Price: Price{Serverless: 2.72}},
	}
	c, err := Pick(gpus, 18, "AMPERE_80", 1)
	if err != nil {
		t.Fatal(err)
	}
	if c.Pool.ID != "AMPERE_80" {
		t.Fatalf("got %s", c.Pool.ID)
	}
}

func TestPickMultiGPU(t *testing.T) {
	gpus := []GPU{
		{ID: "4090", Name: "4090", Pool: ptr("ADA_24"), Memory: 24, Availability: "HIGH", Price: Price{Serverless: 1.10}},
	}
	c, err := Pick(gpus, 50, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if c.GPUCount < 3 {
		t.Fatalf("expected >=3 GPUs, got %d", c.GPUCount)
	}
}

func TestOpenAIURL(t *testing.T) {
	got := OpenAIURL("abc123")
	if got != "https://api.runpod.ai/v2/abc123/openai/v1" {
		t.Fatalf("%s", got)
	}
}

func TestListFittingSortedByPrice(t *testing.T) {
	gpus := []GPU{
		{ID: "A100", Name: "A100", Pool: ptr("AMPERE_80"), Memory: 80, Availability: "HIGH", Price: Price{Serverless: 2.72}},
		{ID: "4090", Name: "4090", Pool: ptr("ADA_24"), Memory: 24, Availability: "HIGH", Price: Price{Serverless: 1.10}},
		{ID: "A40", Name: "A40", Pool: ptr("AMPERE_48"), Memory: 48, Availability: "HIGH", Price: Price{Serverless: 1.50}},
		{ID: "tiny", Name: "tiny", Pool: ptr("AMPERE_16"), Memory: 16, Availability: "HIGH", Price: Price{Serverless: 0.40}},
		{ID: "oos", Name: "oos", Pool: ptr("DEAD"), Memory: 80, Availability: "NONE", Price: Price{Serverless: 0.10}},
	}
	fit := ListFitting(gpus, 23.5)
	if len(fit) != 3 {
		t.Fatalf("want 3 fitting, got %d %#v", len(fit), fit)
	}
	if fit[0].ID != "ADA_24" || fit[1].ID != "AMPERE_48" || fit[2].ID != "AMPERE_80" {
		t.Fatalf("price order: %v %v %v", fit[0].ID, fit[1].ID, fit[2].ID)
	}
}

func TestFittingOptionsPrefersLargerAfterRecommended(t *testing.T) {
	gpus := []GPU{
		{ID: "a", Name: "a", Pool: ptr("CHEAP_24"), Memory: 24, Availability: "HIGH", Price: Price{Serverless: 0.50}},
		{ID: "b", Name: "b", Pool: ptr("MID_24B"), Memory: 24, Availability: "HIGH", Price: Price{Serverless: 0.60}},
		{ID: "c", Name: "c", Pool: ptr("SAFE_48"), Memory: 48, Availability: "HIGH", Price: Price{Serverless: 1.20}},
		{ID: "d", Name: "d", Pool: ptr("BIG_80"), Memory: 80, Availability: "HIGH", Price: Price{Serverless: 2.00}},
		{ID: "e", Name: "e", Pool: ptr("HUGE_80B"), Memory: 80, Availability: "HIGH", Price: Price{Serverless: 2.50}},
		{ID: "f", Name: "f", Pool: ptr("XL_96"), Memory: 96, Availability: "HIGH", Price: Price{Serverless: 3.00}},
	}
	opts := FittingOptions(gpus, 20, 4)
	if len(opts) != 4 {
		t.Fatalf("got %d", len(opts))
	}
	if opts[0].ID != "CHEAP_24" {
		t.Fatalf("recommended %s", opts[0].ID)
	}
	// Remaining should include larger VRAM before same-size mid price filler.
	ids := []string{opts[1].ID, opts[2].ID, opts[3].ID}
	joined := ids[0] + "," + ids[1] + "," + ids[2]
	for _, want := range []string{"SAFE_48", "BIG_80"} {
		if !containsID(ids, want) {
			t.Fatalf("expected larger pool %s in %s", want, joined)
		}
	}
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
