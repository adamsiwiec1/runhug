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
