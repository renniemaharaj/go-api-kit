package kit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/renniemaharaj/go-api-kit/kit"
)

func TestHandleUnmarshalJSONDecodesRequestBody(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}

	handler := kit.NewHandler(func(h kit.Handle) {
		var body input
		if err := h.UnmarshalJSON(&body); err != nil {
			h.Error.Internal(err.Error())
			return
		}
		h.Text(http.StatusOK, body.Name)
	})

	response := request(t, handler, http.MethodPost, "/", `{"name":"Ada"}`)
	if response.Code != http.StatusOK || response.Body.String() != "Ada" {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
}

func TestHandleJSONWritesEncodedResponse(t *testing.T) {
	type output struct {
		ID int `json:"id"`
	}
	handler := kit.NewHandler(func(h kit.Handle) {
		if err := h.JSON(http.StatusCreated, output{ID: 42}); err != nil {
			h.Error.Internal(err.Error())
		}
	})

	response := request(t, handler, http.MethodGet, "/", "")
	if response.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d", response.Code, http.StatusCreated)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("got Content-Type %q", got)
	}
	var got output
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != 42 {
		t.Fatalf("got response %#v", got)
	}
}

func TestHandleUnmarshalJSONRejectsMultipleValues(t *testing.T) {
	handler := kit.NewHandler(func(h kit.Handle) {
		var value any
		if err := h.UnmarshalJSON(&value); err == nil {
			h.Error.Internal("expected decode error")
			return
		}
		h.Error.BadRequest("invalid body")
	})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{} {}`)))
	if response.Code != http.StatusBadRequest || response.Body.String() != "invalid body\n" {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
}

func TestHandleResponseHelpers(t *testing.T) {
	tests := []struct {
		name         string
		handle       kit.HandlerFunc
		wantStatus   int
		wantType     string
		wantBody     string
		wantLocation string
	}{
		{
			name:       "Text",
			handle:     func(h kit.Handle) { h.Text(http.StatusAccepted, "plain") },
			wantStatus: http.StatusAccepted,
			wantType:   "text/plain; charset=utf-8",
			wantBody:   "plain",
		},
		{
			name:       "HTML",
			handle:     func(h kit.Handle) { h.HTML(http.StatusOK, "<p>hello</p>") },
			wantStatus: http.StatusOK,
			wantType:   "text/html; charset=utf-8",
			wantBody:   "<p>hello</p>",
		},
		{
			name:       "Status",
			handle:     func(h kit.Handle) { h.Status(http.StatusAccepted) },
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "NoContent",
			handle:     func(h kit.Handle) { h.NoContent() },
			wantStatus: http.StatusNoContent,
		},
		{
			name:         "Redirect",
			handle:       func(h kit.Handle) { h.Redirect("/next", http.StatusSeeOther) },
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/next",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, kit.NewHandler(test.handle), http.MethodGet, "/", "")
			if response.Code != test.wantStatus {
				t.Fatalf("got status %d, want %d", response.Code, test.wantStatus)
			}
			if got := response.Header().Get("Content-Type"); test.wantType != "" && got != test.wantType {
				t.Fatalf("got Content-Type %q, want %q", got, test.wantType)
			}
			if test.wantBody != "" && response.Body.String() != test.wantBody {
				t.Fatalf("got body %q, want %q", response.Body.String(), test.wantBody)
			}
			if got := response.Header().Get("Location"); got != test.wantLocation {
				t.Fatalf("got Location %q, want %q", got, test.wantLocation)
			}
		})
	}
}

func TestHandleErrorResponses(t *testing.T) {
	tests := []struct {
		name       string
		write      func(kit.ErrorAPI)
		wantStatus int
	}{
		{"BadRequest", func(e kit.ErrorAPI) { e.BadRequest("problem") }, http.StatusBadRequest},
		{"Unauthorized", func(e kit.ErrorAPI) { e.Unauthorized("problem") }, http.StatusUnauthorized},
		{"Forbidden", func(e kit.ErrorAPI) { e.Forbidden("problem") }, http.StatusForbidden},
		{"NotFound", func(e kit.ErrorAPI) { e.NotFound("problem") }, http.StatusNotFound},
		{"Internal", func(e kit.ErrorAPI) { e.Internal("problem") }, http.StatusInternalServerError},
		{"Status", func(e kit.ErrorAPI) { e.Status(http.StatusConflict, "problem") }, http.StatusConflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, kit.NewHandler(func(h kit.Handle) { test.write(h.Error) }), http.MethodGet, "/", "")
			if response.Code != test.wantStatus || response.Body.String() != "problem\n" {
				t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
			}
		})
	}
}

func TestHandleHeadersAndCookies(t *testing.T) {
	handler := kit.NewHandler(func(h kit.Handle) {
		h.Header.Set("X-Request-Value", h.Header.Get("X-Request"))
		h.Header.Add("X-Values", "first")
		h.Header.Add("X-Values", "second")
		h.Header.Set("X-Delete", "removed")
		h.Header.Del("X-Delete")
		cookie, err := h.Cookie("incoming")
		if err != nil {
			h.Error.BadRequest("missing cookie")
			return
		}
		h.SetCookie(&http.Cookie{Name: "session", Value: "abc", Path: "/"})
		h.Text(http.StatusOK, cookie.Value)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request", "present")
	req.AddCookie(&http.Cookie{Name: "incoming", Value: "cookie-value"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)

	if response.Code != http.StatusOK || response.Body.String() != "cookie-value" {
		t.Fatalf("got status %d and body %q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("X-Request-Value"); got != "present" {
		t.Fatalf("got X-Request-Value %q", got)
	}
	if got := response.Header().Values("X-Values"); len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("got X-Values %#v", got)
	}
	if got := response.Header().Get("X-Delete"); got != "" {
		t.Fatalf("got deleted header %q", got)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "session" || cookies[0].Value != "abc" {
		t.Fatalf("got response cookies %#v", cookies)
	}
}

func TestHandleExposesStandardRequestAndWriter(t *testing.T) {
	handler := kit.NewHandler(func(h kit.Handle) {
		if h.Request().Method != http.MethodPatch || h.Path() != "/resource" {
			h.Error.BadRequest("wrong request")
			return
		}
		h.Writer().Header().Set("X-Direct", "yes")
		h.NoContent()
	})

	response := request(t, handler, http.MethodPatch, "/resource", "")
	if response.Code != http.StatusNoContent || response.Header().Get("X-Direct") != "yes" {
		t.Fatalf("got status %d and headers %#v", response.Code, response.Header())
	}
}
