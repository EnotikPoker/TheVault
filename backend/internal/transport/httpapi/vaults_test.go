package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"thevault/backend/internal/session"
	"thevault/backend/internal/vault"
)

func TestGameLifecycle(t *testing.T) {
	fixture := newAPIFixture(t)
	createdResponse := request(t, fixture.handler, "POST", "/api/v1/vaults", "", http.StatusCreated)
	created := readState(t, createdResponse)
	path := "/api/v1/vaults/" + created.ID
	if createdResponse.Header().Get("Location") != path {
		t.Fatal("creation did not return the session location")
	}
	if created.Status != session.Locked || created.Attempts != 0 || created.ElapsedMs != 0 || created.Switches != nil || created.History == nil || len(created.History) != 0 {
		t.Fatalf("unexpected initial state: %+v", created)
	}
	for index, id := range []vault.SystemID{vault.SystemPower, vault.SystemSync, vault.SystemAccess} {
		if indicator := created.Indicators[index]; indicator.ID != id || indicator.Passed != nil {
			t.Fatalf("initial indicator should be neutral: %+v", indicator)
		}
	}
	assertNoHiddenData(t, createdResponse.Body.Bytes(), false)
	initial := readState(t, request(t, fixture.handler, "GET", path, "", http.StatusOK))
	if !reflect.DeepEqual(initial, created) {
		t.Fatal("reading a new session changed its state")
	}
	fixture.elapsedMs.Store(1500)
	for attempt := 1; attempt <= 2; attempt++ {
		recorder := request(t, fixture.handler, "POST", path+"/checks", combinationBody(t, fixture.failing), http.StatusOK)
		checked := readState(t, recorder)
		if checked.Attempts != attempt || len(checked.History) != attempt || checked.ElapsedMs != 1500 || checked.Switches == nil || *checked.Switches != fixture.failing {
			t.Fatalf("unexpected check result: %+v", checked)
		}
		last := checked.History[attempt-1]
		if last.Attempt != attempt || last.Switches != fixture.failing || last.ElapsedMs != 1500 || !reflect.DeepEqual(last.Indicators, checked.Indicators) {
			t.Fatalf("history does not match the submitted check: %+v", last)
		}
		assertNoHiddenData(t, recorder.Body.Bytes(), false)
		fetched := readState(t, request(t, fixture.handler, "GET", path, "", http.StatusOK))
		if !reflect.DeepEqual(fetched, checked) {
			t.Fatal("GET did not restore the full history")
		}
	}
	fixture.elapsedMs.Store(5000)
	openedResponse := request(t, fixture.handler, "POST", path+"/checks", combinationBody(t, fixture.winning), http.StatusOK)
	opened := readState(t, openedResponse)
	if opened.Status != session.Opened || opened.Attempts != 3 || len(opened.History) != 3 || opened.ElapsedMs != 5000 || len(opened.Explanations) != vault.SystemCount {
		t.Fatalf("unexpected opened state: %+v", opened)
	}
	if opened.OpenedAt == nil || !opened.OpenedAt.Equal(created.CreatedAt.Add(5*time.Second)) {
		t.Fatal("incorrect opening time")
	}
	for index, indicator := range opened.Indicators {
		explanation := opened.Explanations[index]
		if indicator.Passed == nil || !*indicator.Passed || explanation.ID != indicator.ID || explanation.Condition == "" || explanation.Observation == "" {
			t.Fatalf("missing solution feedback for %s", indicator.ID)
		}
	}
	assertNoHiddenData(t, openedResponse.Body.Bytes(), true)
	fixture.elapsedMs.Store(3_600_000)
	conflict := request(t, fixture.handler, "POST", path+"/checks", combinationBody(t, fixture.winning), http.StatusConflict)
	assertError(t, conflict, "already_opened")
	fetched := readState(t, request(t, fixture.handler, "GET", path, "", http.StatusOK))
	if !reflect.DeepEqual(fetched, opened) {
		t.Fatal("opened session changed after time passed or a rejected request")
	}
}

func TestInvalidChecksDoNotSpendAttempts(t *testing.T) {
	fixture := newAPIFixture(t)
	created := readState(t, request(t, fixture.handler, "POST", "/api/v1/vaults", "", http.StatusCreated))
	path := "/api/v1/vaults/" + created.ID
	cases := []struct{ name, body string }{
		{"empty", ""},
		{"invalid JSON", "{"},
		{"root null", "null"},
		{"root array", "[]"},
		{"missing switches", "{}"},
		{"null switches", `{"switches":null}`},
		{"short array", `{"switches":[false,false,false,false]}`},
		{"long array", `{"switches":[false,false,false,false,false,false]}`},
		{"null item", `{"switches":[false,false,null,false,false]}`},
		{"number item", `{"switches":[false,false,1,false,false]}`},
		{"string item", `{"switches":[false,false,"true",false,false]}`},
		{"wrong shape", `{"switches":{"S1":true}}`},
		{"unknown field", `{"switches":[false,false,false,false,false],"seed":42}`},
		{"trailing object", `{"switches":[false,false,false,false,false]} {}`},
		{"trailing garbage", `{"switches":[false,false,false,false,false]} garbage`},
		{"oversized", strings.Repeat(" ", 4097) + `{"switches":[false,false,false,false,false]}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			recorder := request(t, fixture.handler, "POST", path+"/checks", tt.body, http.StatusBadRequest)
			assertError(t, recorder, "invalid_request")
			got := readState(t, request(t, fixture.handler, "GET", path, "", http.StatusOK))
			if !reflect.DeepEqual(got, created) {
				t.Fatal("invalid input changed the session")
			}
		})
	}
}

func TestCreateRequestContract(t *testing.T) {
	fixture := newAPIFixture(t)
	for _, body := range []string{"", " \n ", "{}", "{ \n }"} {
		request(t, fixture.handler, "POST", "/api/v1/vaults", body, http.StatusCreated)
	}
	for _, body := range []string{"null", "[]", "true", `{"seed":42}`, "{} {}", "{", strings.Repeat(" ", 4097) + "{}"} {
		recorder := request(t, fixture.handler, "POST", "/api/v1/vaults", body, http.StatusBadRequest)
		assertError(t, recorder, "invalid_request")
	}
}

func TestMissingSessions(t *testing.T) {
	fixture := newAPIFixture(t)
	for _, method := range []string{"GET", "POST"} {
		path := "/api/v1/vaults/missing"
		if method == "POST" {
			path += "/checks"
		}
		recorder := request(t, fixture.handler, method, path, combinationBody(t, fixture.failing), http.StatusNotFound)
		assertError(t, recorder, "not_found")
	}
}

func TestInternalErrorsHideDetails(t *testing.T) {
	handler := NewHandler(unavailableService{})
	for _, endpoint := range []struct{ method, path, body string }{
		{"POST", "/api/v1/vaults", ""},
		{"GET", "/api/v1/vaults/test", ""},
		{"POST", "/api/v1/vaults/test/checks", `{"switches":[false,false,false,false,false]}`},
	} {
		recorder := request(t, handler, endpoint.method, endpoint.path, endpoint.body, http.StatusInternalServerError)
		assertError(t, recorder, "internal_error")
		if strings.Contains(recorder.Body.String(), "private storage failure") {
			t.Fatal("internal error details appeared in the response")
		}
	}
}

func TestHealthAndMethodRouting(t *testing.T) {
	handler := NewHandler(unavailableService{})
	health := request(t, handler, "GET", "/api/health", "", http.StatusOK)
	if strings.TrimSpace(health.Body.String()) != `{"status":"ok"}` {
		t.Fatal("health endpoint changed")
	}
	for _, endpoint := range []struct{ method, path, allow string }{
		{"GET", "/api/v1/vaults", "POST"},
		{"POST", "/api/v1/vaults/test", "GET"},
		{"GET", "/api/v1/vaults/test/checks", "POST"},
	} {
		recorder := perform(handler, endpoint.method, endpoint.path, "")
		if recorder.Code != http.StatusMethodNotAllowed || !strings.Contains(recorder.Header().Get("Allow"), endpoint.allow) {
			t.Fatalf("incorrect method routing: %d %s", recorder.Code, recorder.Header().Get("Allow"))
		}
	}
}

func TestConcurrentHTTPChecks(t *testing.T) {
	fixture := newAPIFixture(t)
	created := readState(t, request(t, fixture.handler, "POST", "/api/v1/vaults", "", http.StatusCreated))
	path := "/api/v1/vaults/" + created.ID
	body := combinationBody(t, fixture.failing)
	const requests = 64
	responses := make(chan *httptest.ResponseRecorder, requests)
	for range requests {
		go func() { responses <- perform(fixture.handler, "POST", path+"/checks", body) }()
	}
	seen := make(map[int]bool)
	for range requests {
		recorder := <-responses
		if recorder.Code != http.StatusOK {
			t.Fatalf("concurrent check: %d %s", recorder.Code, recorder.Body.String())
		}
		state := readState(t, recorder)
		if state.Attempts < 1 || state.Attempts > requests || seen[state.Attempts] || len(state.History) != state.Attempts {
			t.Fatalf("inconsistent concurrent response: %+v", state)
		}
		seen[state.Attempts] = true
	}
	final := readState(t, request(t, fixture.handler, "GET", path, "", http.StatusOK))
	if final.Attempts != requests || len(final.History) != requests {
		t.Fatal("HTTP checks were lost")
	}
}

func TestConcurrentHTTPOpening(t *testing.T) {
	fixture := newAPIFixture(t)
	created := readState(t, request(t, fixture.handler, "POST", "/api/v1/vaults", "", http.StatusCreated))
	path := "/api/v1/vaults/" + created.ID
	body := combinationBody(t, fixture.winning)
	const requests = 32
	responses := make(chan *httptest.ResponseRecorder, requests)
	for range requests {
		go func() { responses <- perform(fixture.handler, "POST", path+"/checks", body) }()
	}
	accepted := 0
	for range requests {
		recorder := <-responses
		switch recorder.Code {
		case http.StatusOK:
			accepted++
		case http.StatusConflict:
			assertError(t, recorder, "already_opened")
		default:
			t.Fatalf("concurrent opening: %d %s", recorder.Code, recorder.Body.String())
		}
	}
	final := readState(t, request(t, fixture.handler, "GET", path, "", http.StatusOK))
	if accepted != 1 || final.Attempts != 1 || len(final.History) != 1 || final.Status != session.Opened {
		t.Fatalf("multiple HTTP requests opened the session: %d, %+v", accepted, final)
	}
}

func TestResponseUsesUTCAndHidesLockedExplanations(t *testing.T) {
	createdAt := time.Date(2026, 10, 28, 10, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	explanations := [vault.SystemCount]vault.Explanation{{Condition: "private rule"}}
	body, err := json.Marshal(response(session.Snapshot{
		CreatedAt: createdAt, Status: session.Locked, OpenedAt: &createdAt, Explanations: &explanations,
	}))
	if err != nil {
		t.Fatal(err)
	}
	assertNoHiddenData(t, body, false)
	if !strings.Contains(string(body), `"createdAt":"2026-10-28T07:00:00Z"`) || strings.Contains(string(body), "openedAt") {
		t.Fatalf("unexpected timestamps: %s", body)
	}
}

type apiFixture struct {
	handler   http.Handler
	elapsedMs atomic.Int64
	winning   vault.Combination
	failing   vault.Combination
}

func newAPIFixture(t *testing.T) *apiFixture {
	t.Helper()
	puzzle, err := vault.Generate(42)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &apiFixture{}
	for state := range 1 << vault.SwitchCount {
		var combination vault.Combination
		for index := range combination {
			combination[index] = state&(1<<index) != 0
		}
		if puzzle.Check(combination).Opened {
			fixture.winning = combination
		} else {
			fixture.failing = combination
		}
	}
	start := time.Date(2026, 10, 28, 10, 0, 0, 0, time.UTC)
	service := session.NewService(session.NewMemoryStore(), session.Options{
		Now:      func() time.Time { return start.Add(time.Duration(fixture.elapsedMs.Load()) * time.Millisecond) },
		NewVault: func() (vault.Vault, error) { return puzzle, nil },
	})
	fixture.handler = NewHandler(service)
	return fixture
}

func combinationBody(t *testing.T, switches vault.Combination) string {
	t.Helper()
	body, err := json.Marshal(struct {
		Switches vault.Combination `json:"switches"`
	}{switches})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func perform(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, req)
	return recorder
}

func request(t *testing.T, handler http.Handler, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	recorder := perform(handler, method, path, body)
	if recorder.Code != status {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, recorder.Code, status, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" || recorder.Header().Get("Cache-Control") != "no-store" || !json.Valid(recorder.Body.Bytes()) {
		t.Fatalf("invalid JSON response or headers: %s", recorder.Body.String())
	}
	return recorder
}

func readState(t *testing.T, recorder *httptest.ResponseRecorder) stateResponse {
	t.Helper()
	var state stateResponse
	decoder := json.NewDecoder(strings.NewReader(recorder.Body.String()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

func assertError(t *testing.T, recorder *httptest.ResponseRecorder, code string) {
	t.Helper()
	var body errorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Error.Code != code || body.Error.Message == "" {
		t.Fatalf("unexpected error response: %s", recorder.Body.String())
	}
	assertNoHiddenData(t, recorder.Body.Bytes(), false)
}

func assertNoHiddenData(t *testing.T, data []byte, opened bool) {
	t.Helper()
	var body any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	var visit func(any)
	visit = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				switch strings.ToLower(key) {
				case "seed", "rules", "puzzle", "vault":
					t.Fatalf("private field %q appeared in response", key)
				case "explanations", "condition", "observation":
					if !opened {
						t.Fatalf("explanation field %q appeared before opening", key)
					}
				}
				visit(child)
			}
		case []any:
			for _, child := range value {
				visit(child)
			}
		}
	}
	visit(body)
}

type unavailableService struct{}

func (unavailableService) Create() (session.Snapshot, error) {
	return session.Snapshot{}, errors.New("private storage failure")
}

func (unavailableService) Get(string) (session.Snapshot, error) {
	return session.Snapshot{}, errors.New("private storage failure")
}

func (unavailableService) Check(string, vault.Combination) (session.Snapshot, error) {
	return session.Snapshot{}, errors.New("private storage failure")
}
