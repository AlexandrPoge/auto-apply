package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	hhAuthorizeURL = "https://hh.ru/oauth/authorize"
	hhTokenURL     = "https://api.hh.ru/token"
	hhVacanciesURL = "https://api.hh.ru/vacancies"
	oauthStateTTL  = 10 * time.Minute
)

type hhClient struct {
	cfg        config
	httpClient *http.Client
	states     map[string]time.Time
	mu         sync.Mutex
}

type hhToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	CreatedAt    time.Time `json:"created_at"`
}

type hhStatus struct {
	Configured bool `json:"configured"`
	Connected  bool `json:"connected"`
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

func newHHClient(cfg config, client *http.Client) *hhClient {
	return &hhClient{cfg: cfg, httpClient: client, states: make(map[string]time.Time)}
}

func (h *hhClient) configured() bool {
	return configuredValue(h.cfg.HHClientID) && configuredValue(h.cfg.HHClientSecret)
}

func configuredValue(value string) bool {
	return value != "" && value != "replace_me"
}

func (h *hhClient) tokenPath() string {
	return filepath.Join(h.cfg.LocalStoragePath, "hh-token.json")
}

func (h *hhClient) loadToken() (hhToken, error) {
	var token hhToken
	raw, err := os.ReadFile(h.tokenPath())
	if err != nil {
		return token, err
	}
	if err := json.Unmarshal(raw, &token); err != nil {
		return token, fmt.Errorf("не удалось прочитать локальный токен hh.ru: %w", err)
	}
	if token.AccessToken == "" {
		return token, errors.New("локальный токен hh.ru пуст")
	}
	return token, nil
}

func (h *hhClient) saveToken(token hhToken) error {
	if err := os.MkdirAll(h.cfg.LocalStoragePath, 0o700); err != nil {
		return fmt.Errorf("не удалось создать каталог токена: %w", err)
	}
	token.CreatedAt = time.Now().UTC()
	raw, err := json.Marshal(token)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(h.cfg.LocalStoragePath, "hh-token-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, h.tokenPath()); err != nil {
		return err
	}
	return nil
}

func (h *hhClient) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "используйте GET")
		return
	}
	_, err := h.loadToken()
	writeJSON(w, http.StatusOK, hhStatus{Configured: h.configured(), Connected: err == nil})
}

func (h *hhClient) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "используйте GET")
		return
	}
	if !h.configured() {
		writeError(w, http.StatusServiceUnavailable, "добавьте HH_CLIENT_ID и HH_CLIENT_SECRET в .env")
		return
	}
	state, err := randomState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось подготовить безопасный вход")
		return
	}
	h.mu.Lock()
	for value, expiresAt := range h.states {
		if time.Now().After(expiresAt) {
			delete(h.states, value)
		}
	}
	h.states[state] = time.Now().Add(oauthStateTTL)
	h.mu.Unlock()

	http.SetCookie(w, &http.Cookie{Name: "hh_oauth_state", Value: state, Path: "/auth/hh", MaxAge: int(oauthStateTTL.Seconds()), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	query := url.Values{
		"response_type": {"code"},
		"client_id":     {h.cfg.HHClientID},
		"redirect_uri":  {h.cfg.HHRedirectURL},
		"state":         {state},
	}
	http.Redirect(w, r, hhAuthorizeURL+"?"+query.Encode(), http.StatusFound)
}

func (h *hhClient) handleCallback(w http.ResponseWriter, r *http.Request) {
	if err := h.validateState(r); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "hh.ru не вернул код авторизации")
		return
	}
	token, err := h.exchangeCode(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "не удалось подключить hh.ru: "+err.Error())
		return
	}
	if err := h.saveToken(token); err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось безопасно сохранить токен: "+err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "hh_oauth_state", Value: "", Path: "/auth/hh", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/?hh=connected", http.StatusFound)
}

func (h *hhClient) validateState(r *http.Request) error {
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("hh_oauth_state")
	if err != nil || state == "" || cookie.Value != state {
		return errors.New("проверка входа hh.ru не пройдена; начните подключение заново")
	}
	h.mu.Lock()
	expiresAt, ok := h.states[state]
	delete(h.states, state)
	h.mu.Unlock()
	if !ok || time.Now().After(expiresAt) {
		return errors.New("время подключения истекло; начните подключение заново")
	}
	return nil
}

func (h *hhClient) exchangeCode(ctx context.Context, code string) (hhToken, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {h.cfg.HHClientID},
		"client_secret": {h.cfg.HHClientSecret},
		"code":          {code},
		"redirect_uri":  {h.cfg.HHRedirectURL},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hhTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return hhToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HH-User-Agent", h.cfg.HHUserAgent)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return hhToken{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return hhToken{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return hhToken{}, fmt.Errorf("hh.ru вернул %s: %s", resp.Status, compactError(raw))
	}
	var token hhToken
	if err := json.Unmarshal(raw, &token); err != nil {
		return hhToken{}, errors.New("hh.ru вернул неожиданный ответ")
	}
	if token.AccessToken == "" {
		return hhToken{}, errors.New("hh.ru не вернул access token")
	}
	return token, nil
}

func (h *hhClient) handleVacancies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "используйте POST")
		return
	}
	if !h.configured() {
		writeError(w, http.StatusServiceUnavailable, "сначала добавьте учётные данные OAuth hh.ru в .env")
		return
	}
	token, err := h.loadToken()
	if err != nil {
		writeError(w, http.StatusUnauthorized, "сначала подключите аккаунт hh.ru")
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
	result, err := h.searchVacancies(r.Context(), token.AccessToken, input)
	if err != nil {
		writeError(w, http.StatusBadGateway, "не удалось получить вакансии: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *hhClient) searchVacancies(ctx context.Context, accessToken string, input vacancySearchRequest) (vacancySearchResponse, error) {
	query := url.Values{"text": {input.Text}, "per_page": {strconv.Itoa(input.PerPage)}, "order_by": {"publication_time"}}
	if input.Area != "" {
		if _, err := strconv.Atoi(input.Area); err != nil {
			return vacancySearchResponse{}, errors.New("ID региона hh.ru должен состоять из цифр")
		}
		query.Set("area", input.Area)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hhVacanciesURL+"?"+query.Encode(), nil)
	if err != nil {
		return vacancySearchResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("HH-User-Agent", h.cfg.HHUserAgent)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return vacancySearchResponse{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return vacancySearchResponse{}, err
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

func randomState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
