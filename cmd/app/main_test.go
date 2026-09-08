package main

import (
	"encoding/json"
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
		"Ссылка на вакансию hh.ru",
		"Одобрить и открыть hh.ru",
		"localStorage",
	} {
		if !strings.Contains(body, text) {
			t.Errorf("home page does not contain %q", text)
		}
	}
}

func TestBuildTemplateLetterUsesOnlyInputText(t *testing.T) {
	letter := buildTemplateLetter(letterRequest{
		CandidateProfile: "Разрабатываю сервисы на Go и PostgreSQL. Второе предложение не должно попасть в письмо.",
		VacancyText:      "Вакансия: Go developer\nНужен опыт работы с API.",
	})
	for _, text := range []string{"Go developer", "Разрабатываю сервисы на Go и PostgreSQL."} {
		if !strings.Contains(letter, text) {
			t.Errorf("letter does not contain %q: %s", text, letter)
		}
	}
	if strings.Contains(letter, "Второе предложение") {
		t.Errorf("letter contains profile text beyond the first sentence: %s", letter)
	}
}

func TestLetterEndpointUsesLocalTemplate(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/letter", strings.NewReader(`{"candidate_profile":"Опыт Go.","vacancy_text":"Вакансия: Backend developer"}`))
	request.Header.Set("Content-Type", "application/json")
	handleLetter(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	var response letterResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Source != "local_template" || !strings.Contains(response.Letter, "Backend developer") {
		t.Fatalf("unexpected response: %#v", response)
	}
}
