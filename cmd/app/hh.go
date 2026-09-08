package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const hhVacanciesURL = "https://api.hh.ru/vacancies"

type vacancyClient struct {
	httpClient   *http.Client
	userAgent    string
	vacanciesURL string
}

type vacancySearchRequest struct {
	Text    string `json:"text"`
	Area    string `json:"area"`
	PerPage int    `json:"per_page"`
}

type vacancySearchResponse struct {
	Found int         `json:"found"`
	Items []hhVacancy `json:"items"`
}

type hhVacancy struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AlternateURL string `json:"alternate_url"`
	PublishedAt  string `json:"published_at"`
	Employer     struct {
		Name string `json:"name"`
	} `json:"employer"`
	Area struct {
		Name string `json:"name"`
	} `json:"area"`
	Salary *struct {
		From     *int   `json:"from"`
		To       *int   `json:"to"`
		Currency string `json:"currency"`
	} `json:"salary"`
	Snippet struct {
		Requirement    string `json:"requirement"`
		Responsibility string `json:"responsibility"`
	} `json:"snippet"`
}

func newVacancyClient(userAgent string, client *http.Client) *vacancyClient {
	return &vacancyClient{httpClient: client, userAgent: userAgent, vacanciesURL: hhVacanciesURL}
}

func (h *vacancyClient) handleVacancies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "используйте POST")
		return
	}
	var input vacancySearchRequest
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "неверный JSON: "+err.Error())
		return
	}
	input.Text = strings.TrimSpace(input.Text)
	input.Area = strings.TrimSpace(input.Area)
	if input.Text == "" {
		writeError(w, http.StatusBadRequest, "укажите поисковый запрос")
		return
	}
	if input.PerPage < 1 || input.PerPage > 20 {
		input.PerPage = 10
	}
	result, err := h.searchVacancies(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadGateway, "не удалось получить вакансии: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *vacancyClient) searchVacancies(ctx context.Context, input vacancySearchRequest) (vacancySearchResponse, error) {
	query := url.Values{"text": {input.Text}, "per_page": {strconv.Itoa(input.PerPage)}, "order_by": {"publication_time"}}
	if input.Area != "" {
		if _, err := strconv.Atoi(input.Area); err != nil {
			return vacancySearchResponse{}, errors.New("ID региона hh.ru должен состоять из цифр")
		}
		query.Set("area", input.Area)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.vacanciesURL+"?"+query.Encode(), nil)
	if err != nil {
		return vacancySearchResponse{}, err
	}
	req.Header.Set("HH-User-Agent", h.userAgent)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return vacancySearchResponse{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return vacancySearchResponse{}, err
	}
	if resp.StatusCode == http.StatusForbidden {
		return vacancySearchResponse{}, errors.New("hh.ru запросил капчу для публичного поиска; откройте вакансию на hh.ru и подтвердите поиск вручную")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return vacancySearchResponse{}, fmt.Errorf("hh.ru вернул %s: %s", resp.Status, compactError(raw))
	}
	var result vacancySearchResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return vacancySearchResponse{}, errors.New("hh.ru вернул неожиданный список вакансий")
	}
	return result, nil
}
