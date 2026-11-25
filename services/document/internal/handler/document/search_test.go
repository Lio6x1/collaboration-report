package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

type searchPageHTTPFixture struct{ calls int }

func (s *searchPageHTTPFixture) Page(_ context.Context, _ *logic.Actor, query domain.SearchQuery, _ string, _ logic.BindOutput) (dto.SearchResultPage, error) {
	s.calls++
	return dto.SearchResultPage{Items: []dto.SearchResult{}}, nil
}

func TestSearchHTTPStrictParametersAndCookieNeverReachUsecase(t *testing.T) {
	for _, query := range []string{"q=%zz", "q=x;y=z", "q=a&q=b", "q=a&scope_hash=fake", "q=a&session_ref=fake", "q=a&limit=01"} {
		usecase := &searchPageHTTPFixture{}
		handler := NewSearchPages(NewIdentity(nil), nil, usecase)
		request := httptest.NewRequest("GET", "/v1/search?"+query, nil)
		response := httptest.NewRecorder()
		handler.Page(response, request)
		if response.Code != 400 || usecase.calls != 0 {
			t.Fatalf("invalid query passed: %s status=%d", query, response.Code)
		}
	}
	for _, cookie := range []string{"__Host-collabSession=", "__Host-collabSession=bad", "__Host-collabSession=x; __Host-collabSession=y"} {
		usecase := &searchPageHTTPFixture{}
		handler := NewSearchPages(NewIdentity(nil), nil, usecase)
		request := httptest.NewRequest("GET", "/v1/search?q=a", nil)
		request.Header.Set("Cookie", cookie)
		response := httptest.NewRecorder()
		handler.Page(response, request)
		if response.Code != 401 || usecase.calls != 0 {
			t.Fatal("invalid supplied session became anonymous")
		}
	}
}

func TestSearchHTTPAnonymousExhaustedEmptyPageNeedsNoFakeOutput(t *testing.T) {
	usecase := &searchPageHTTPFixture{}
	handler := NewSearchPages(NewIdentity(nil), nil, usecase)
	response := httptest.NewRecorder()
	handler.Page(response, httptest.NewRequest("GET", "/v1/search?q=%E6%90%9C%E7%B4%A2", nil))
	if response.Code != 200 || usecase.calls != 1 || !strings.Contains(response.Body.String(), `"items":[]`) || !strings.Contains(response.Body.String(), `"next_cursor":null`) {
		t.Fatalf("empty page: %d %s", response.Code, response.Body.String())
	}
}
