package auth

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devflex/traffoflex/apps/api-service/internal/scope"
)

func TestWorkspaceAllowsAdminToEditAnotherUserData(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(
		repo,
		testConfig(),
	)
	admin := User{ID: "usr_admin", Role: RoleAdmin, Approved: true}
	owner := User{ID: "usr_owner", Role: RoleUser, Approved: true}
	for _, user := range []User{admin, owner} {
		if _, err := repo.SaveUser(
			context.Background(),
			user,
		); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(
		slog.New(slog.NewTextHandler(
			io.Discard,
			nil,
		)),
		service,
	)
	endpoint := h.Workspace(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := scope.OwnerID(r.Context()); got != owner.ID {
			t.Errorf(
				"owner = %q, want %q",
				got,
				owner.ID,
			)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/campaigns/cmp_1",
		nil,
	)
	req.Header.Set(
		"X-TraffoFlex-Act-As",
		owner.ID,
	)
	req = req.WithContext(ContextWithUser(
		req.Context(),
		admin,
	))
	response := httptest.NewRecorder()
	endpoint.ServeHTTP(
		response,
		req,
	)
	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want 204",
			response.Code,
		)
	}
}

func TestWorkspaceRejectsUserImpersonation(t *testing.T) {
	service := NewService(
		NewMemoryRepository(),
		testConfig(),
	)
	h := NewHandler(
		slog.New(slog.NewTextHandler(
			io.Discard,
			nil,
		)),
		service,
	)
	endpoint := h.Workspace(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/campaigns",
		nil,
	)
	req.Header.Set(
		"X-TraffoFlex-Act-As",
		"usr_other",
	)
	req = req.WithContext(ContextWithUser(
		req.Context(),
		User{ID: "usr_user", Role: RoleUser, Approved: true},
	))
	response := httptest.NewRecorder()
	endpoint.ServeHTTP(
		response,
		req,
	)
	if response.Code != http.StatusForbidden {
		t.Fatalf(
			"status = %d, want 403",
			response.Code,
		)
	}
}

func TestWorkspaceRejectsUnknownAdminTarget(t *testing.T) {
	service := NewService(
		NewMemoryRepository(),
		testConfig(),
	)
	h := NewHandler(
		slog.New(slog.NewTextHandler(
			io.Discard,
			nil,
		)),
		service,
	)
	endpoint := h.Workspace(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/campaigns",
		nil,
	)
	req.Header.Set(
		"X-TraffoFlex-Act-As",
		"usr_missing",
	)
	req = req.WithContext(ContextWithUser(
		req.Context(),
		User{ID: "usr_admin", Role: RoleAdmin, Approved: true},
	))
	response := httptest.NewRecorder()
	endpoint.ServeHTTP(
		response,
		req,
	)
	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want 404",
			response.Code,
		)
	}
}

func TestCookieMutationsRequireTrustedOriginAndCSRFHeader(t *testing.T) {
	service := newTestService()
	service.cfg.AllowedOrigin = "https://app.example.test"
	challenge, err := service.RequestOTP(
		context.Background(),
		"admin@example.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.VerifyOTP(
		context.Background(),
		"admin@example.com",
		challenge.OTP,
	)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(
		slog.New(slog.NewTextHandler(
			io.Discard,
			nil,
		)),
		service,
	)
	endpoint := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, test := range []struct {
		name   string
		origin string
		csrf   string
		want   int
	}{
		{name: "missing origin", csrf: "1", want: http.StatusForbidden},
		{name: "wrong origin", origin: "https://other.example.test", csrf: "1", want: http.StatusForbidden},
		{name: "missing header", origin: "https://app.example.test", want: http.StatusForbidden},
		{name: "valid", origin: "https://app.example.test", csrf: "1", want: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPut,
				"/api/campaigns/cmp_1",
				nil,
			)
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.Token})
			if test.origin != "" {
				req.Header.Set(
					"Origin",
					test.origin,
				)
			}
			if test.csrf != "" {
				req.Header.Set(
					"X-TraffoFlex-CSRF",
					test.csrf,
				)
			}
			response := httptest.NewRecorder()
			endpoint.ServeHTTP(
				response,
				req,
			)
			if response.Code != test.want {
				t.Fatalf(
					"status = %d, want %d",
					response.Code,
					test.want,
				)
			}
		})
	}
}
