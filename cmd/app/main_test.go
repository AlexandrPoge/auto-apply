package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomeOffersSafeAutomationModes(t *testing.T) {
	recorder := httptest.NewRecorder()
	handleHome(recorder, httptest.NewRequest("GET", "/", nil))

	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	for _, text := range []string{
		"Очередь на подтверждение",
		"Автоподача по строгим правилам",
		"Макс. откликов в день",
		"localStorage",
	} {
		if !strings.Contains(body, text) {
			t.Errorf("home page does not contain %q", text)
		}
	}
}
