package local

import "testing"

func TestDefaultURL(t *testing.T) {
	if DefaultURL(0) != "http://127.0.0.1:8081/v1" {
		t.Fatalf("%s", DefaultURL(0))
	}
	if DefaultURL(9090) != "http://127.0.0.1:9090/v1" {
		t.Fatalf("%s", DefaultURL(9090))
	}
}
