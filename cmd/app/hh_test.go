package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVacancySearchUsesPublicHHRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("HH-User-Agent"); got != "Auto-Apply test@example.com" {
			t.Errorf("HH-User-Agent = %q", got)
		}
		if got := r.URL.Query().Get("text"); got != "Go developer" {
			t.Errorf("text = %q", got)
		}
		if got := r.URL.Query().Get("area"); got != "1002" {
			t.Errorf("area = %q", got)
		}
		_, _ = w.Write([]byte(`{"found":1,"items":[{"id":"1","name":"Go developer"}]}`))
	}))
	defer server.Close()

	client := newVacancyClient("Auto-Apply test@example.com", server.Client())
	client.vacanciesURL = server.URL
	result, err := client.searchVacancies(t.Context(), vacancySearchRequest{Text: "Go developer", Area: "1002", PerPage: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Found != 1 || len(result.Items) != 1 || result.Items[0].Name != "Go developer" {
		t.Fatalf("unexpected response: %#v", result)
	}
}

func TestVacancySearchRejectsNonNumericArea(t *testing.T) {
	client := newVacancyClient("test", http.DefaultClient)
	if _, err := client.searchVacancies(t.Context(), vacancySearchRequest{Text: "Go", Area: "Minsk"}); err == nil {
		t.Fatal("expected an error for a nonnumeric area")
	}
}
