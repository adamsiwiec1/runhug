package runpod

// OfflineCatalog is a static serverless pool approximation used when no
// Runpod API key is available (recommend / dry sizing). Prices are rough
// USD/hr serverless ballparks and may drift from live catalog.
func OfflineCatalog() []GPU {
	return []GPU{
		{ID: "NVIDIA RTX A4000", Name: "RTX A4000", Pool: strPtr("AMPERE_16"), Memory: 16, Availability: "HIGH", Price: Price{Serverless: 0.22}},
		{ID: "NVIDIA RTX 4090", Name: "RTX 4090", Pool: strPtr("ADA_24"), Memory: 24, Availability: "HIGH", Price: Price{Serverless: 0.44}},
		{ID: "NVIDIA L40", Name: "L40", Pool: strPtr("ADA_48"), Memory: 48, Availability: "MEDIUM", Price: Price{Serverless: 0.99}},
		{ID: "NVIDIA A40", Name: "A40", Pool: strPtr("AMPERE_48"), Memory: 48, Availability: "MEDIUM", Price: Price{Serverless: 0.79}},
		{ID: "NVIDIA A100 80GB", Name: "A100 80GB", Pool: strPtr("AMPERE_80"), Memory: 80, Availability: "LOW", Price: Price{Serverless: 1.89}},
		{ID: "NVIDIA H100 80GB", Name: "H100 80GB", Pool: strPtr("HOPPER_80"), Memory: 80, Availability: "LOW", Price: Price{Serverless: 3.89}},
	}
}

func strPtr(s string) *string { return &s }
