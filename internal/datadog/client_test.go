package datadog

import "testing"

func TestNewClientRequiresKeys(t *testing.T) {
	t.Parallel()

	_, err := NewClient(ClientConfig{})
	if err == nil {
		t.Fatal("expected error when keys are missing")
	}
}

func TestAPIBaseURLForSite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		site string
		want string
	}{
		{name: "default", site: "", want: "https://api.datadoghq.com"},
		{name: "us3", site: "us3.datadoghq.com", want: "https://api.us3.datadoghq.com"},
		{name: "already api host", site: "api.datadoghq.eu", want: "https://api.datadoghq.eu"},
		{name: "app host", site: "app.datadoghq.eu", want: "https://api.datadoghq.eu"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := apiBaseURLForSite(tt.site)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
