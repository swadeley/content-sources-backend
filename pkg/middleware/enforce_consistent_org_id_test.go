package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/content-services/content-sources-backend/pkg/config"
	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	"github.com/stretchr/testify/assert"
)

func encodeIdentity(xrhid identity.XRHID) string {
	jsonIdentity, _ := json.Marshal(xrhid)
	return base64.StdEncoding.EncodeToString(jsonIdentity)
}

func setupTestServer() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = config.CustomHTTPErrorHandler
	e.Use(WrapMiddlewareWithSkipper(identity.EnforceIdentity, SkipMiddleware))
	return e
}

func setupTestRequest(method, url string, xrhid identity.XRHID) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, url, nil)
	rec := httptest.NewRecorder()

	encodedIdentity := encodeIdentity(xrhid)
	req.Header.Set("X-Rh-Identity", encodedIdentity)

	return req, rec
}

func TestEnforceConsistentOrgId_Success(t *testing.T) {
	testOrgId := "test-org-123"
	testAccountId := "test-account-456"

	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: testOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/repositories/", xrhid)

	testHandler := func(c echo.Context) error {
		response := map[string]any{
			"data": []map[string]any{
				{
					"uuid":       "test-uuid-123",
					"name":       "Test Repo",
					"url":        "http://example.com/repo",
					"org_id":     testOrgId, // Same as user's org ID
					"account_id": testAccountId,
				},
			},
		}
		return c.JSON(http.StatusOK, response)
	}

	e.GET("/api/content-sources/v1/repositories/", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestEnforceConsistentOrgId_OrgIdMismatch(t *testing.T) {
	userOrgId := "user-org-456"
	repoOrgId := "repo-org-123"
	testAccountId := "test-account-789"

	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: userOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/repositories/", xrhid)

	testHandler := func(c echo.Context) error {
		response := map[string]any{
			"data": []map[string]any{
				{
					"uuid":       "test-uuid-123",
					"name":       "Test Repo",
					"url":        "http://example.com/repo",
					"org_id":     repoOrgId, // Different from user's org ID
					"account_id": testAccountId,
				},
			},
		}
		return c.JSON(http.StatusOK, response)
	}

	e.GET("/api/content-sources/v1/repositories/", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestEnforceConsistentOrgId_MissingOrgId(t *testing.T) {
	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: "test-account-123",
			Internal: identity.Internal{
				OrgID: "", // Missing org ID
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/repositories/", xrhid)

	testHandler := func(c echo.Context) error {
		response := map[string]any{
			"data": []map[string]any{},
		}
		return c.JSON(http.StatusOK, response)
	}

	e.GET("/api/content-sources/v1/repositories/", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEnforceConsistentOrgId_NonRepositoryEndpoint(t *testing.T) {
	testOrgId := "test-org-123"
	testAccountId := "test-account-456"

	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: testOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/ping", xrhid)

	testHandler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	e.GET("/api/content-sources/v1/ping", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetAccountIdOrgId(t *testing.T) {
	testOrgId := "test-org-123"
	testAccountId := "test-account-456"

	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: testOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/repositories/", xrhid)

	testHandler := func(c echo.Context) error {
		accountId, orgId := getAccountIdOrgId(c)

		assert.Equal(t, testAccountId, accountId)
		assert.Equal(t, testOrgId, orgId)

		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}

	e.GET("/api/content-sources/v1/repositories/", testHandler)

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestEnforceConsistentOrgId_GenericResponse_LowercaseOrgId(t *testing.T) {
	testOrgId := "test-org-123"
	testAccountId := "test-account-456"

	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: testOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/tasks/", xrhid)

	testHandler := func(c echo.Context) error {
		response := map[string]any{
			"data": []map[string]any{
				{
					"uuid":   "task-uuid-123",
					"status": "completed",
					"org_id": testOrgId, // Using lowercase org_id format
					"type":   "introspection",
				},
			},
		}
		return c.JSON(http.StatusOK, response)
	}

	e.GET("/api/content-sources/v1/tasks/", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestEnforceConsistentOrgId_GenericResponse_DirectOrgId(t *testing.T) {
	testOrgId := "test-org-123"
	testAccountId := "test-account-456"

	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: testOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/features/", xrhid)

	testHandler := func(c echo.Context) error {
		response := map[string]any{
			"org_id":       testOrgId, // Direct org_id field
			"feature_list": []string{"feature1", "feature2"},
		}
		return c.JSON(http.StatusOK, response)
	}

	e.GET("/api/content-sources/v1/features/", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestEnforceConsistentOrgId_GenericResponse_MismatchLowercaseOrgId(t *testing.T) {
	userOrgId := "user-org-456"
	responseOrgId := "response-org-123"
	testAccountId := "test-account-789"

	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: userOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/tasks/", xrhid)

	testHandler := func(c echo.Context) error {
		response := map[string]any{
			"data": []map[string]any{
				{
					"uuid":   "task-uuid-123",
					"status": "completed",
					"org_id": responseOrgId, // Different from user's org ID
					"type":   "introspection",
				},
			},
		}
		return c.JSON(http.StatusOK, response)
	}

	e.GET("/api/content-sources/v1/tasks/", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestExtractOrgIds_VariousFormats(t *testing.T) {
	response1 := map[string]any{
		"org_id": "test-org-123",
		"other":  "data",
	}
	orgIds1 := extractOrgIds(response1)
	assert.Equal(t, []string{"test-org-123"}, orgIds1)

	response2 := map[string]any{
		"data": []interface{}{
			map[string]interface{}{
				"uuid":   "item1",
				"org_id": "test-org-789",
			},
			map[string]interface{}{
				"uuid":   "item2",
				"org_id": "test-org-789",
			},
		},
	}
	orgIds2 := extractOrgIds(response2)
	assert.Equal(t, []string{"test-org-789", "test-org-789"}, orgIds2)

	response3 := map[string]any{
		"data": map[string]interface{}{
			"uuid":   "single-item",
			"org_id": "test-org-single",
		},
	}
	orgIds3 := extractOrgIds(response3)
	assert.Equal(t, []string{"test-org-single"}, orgIds3)

	response4 := map[string]any{
		"other": "data",
	}
	orgIds4 := extractOrgIds(response4)
	assert.Empty(t, orgIds4)

	response5 := map[string]any{
		"data": []interface{}{},
	}
	orgIds5 := extractOrgIds(response5)
	assert.Empty(t, orgIds5)
}

func TestEnforceConsistentOrgId_AllowRHELOrgId(t *testing.T) {
	userOrgId := "test-org-123"
	testAccountId := "test-account-456"

	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: userOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/repositories/", xrhid)

	testHandler := func(c echo.Context) error {
		response := map[string]any{
			"data": []map[string]any{
				{
					"uuid":       "rhel-repo-123",
					"name":       "RHEL Repository",
					"url":        "http://example.com/rhel-repo",
					"org_id":     "-1", // RHEL org_id should be allowed
					"account_id": testAccountId,
				},
			},
		}
		return c.JSON(http.StatusOK, response)
	}

	e.GET("/api/content-sources/v1/repositories/", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestEnforceConsistentOrgId_AllowCommunityOrgId(t *testing.T) {
	userOrgId := "test-org-123"
	testAccountId := "test-account-456"

	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: userOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/repositories/", xrhid)

	testHandler := func(c echo.Context) error {
		response := map[string]any{
			"data": []map[string]any{
				{
					"uuid":       "community-repo-123",
					"name":       "Community Repository",
					"url":        "http://example.com/community-repo",
					"org_id":     "-2", // Community org_id should be allowed
					"account_id": testAccountId,
				},
			},
		}
		return c.JSON(http.StatusOK, response)
	}

	e.GET("/api/content-sources/v1/repositories/", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestEnforceConsistentOrgId_MixedOrgIds(t *testing.T) {
	userOrgId := "test-org-123"
	testAccountId := "test-account-456"

	// Create mock identity
	xrhid := identity.XRHID{
		Identity: identity.Identity{
			AccountNumber: testAccountId,
			Internal: identity.Internal{
				OrgID: userOrgId,
			},
			User: &identity.User{Username: "user"},
			Type: "Associate",
		},
	}

	e := setupTestServer()
	req, rec := setupTestRequest(http.MethodGet, "/api/content-sources/v1/repositories/", xrhid)

	testHandler := func(c echo.Context) error {
		response := map[string]any{
			"data": []map[string]any{
				{
					"uuid":       "user-repo-123",
					"name":       "User Repository",
					"url":        "http://example.com/user-repo",
					"org_id":     userOrgId, // User's org_id
					"account_id": testAccountId,
				},
				{
					"uuid":       "rhel-repo-123",
					"name":       "RHEL Repository",
					"url":        "http://example.com/rhel-repo",
					"org_id":     "-1", // RHEL org_id
					"account_id": testAccountId,
				},
				{
					"uuid":       "community-repo-123",
					"name":       "Community Repository",
					"url":        "http://example.com/community-repo",
					"org_id":     "-2", // Community org_id
					"account_id": testAccountId,
				},
			},
		}
		return c.JSON(http.StatusOK, response)
	}

	e.GET("/api/content-sources/v1/repositories/", EnforceConsistentOrgId(testHandler))

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
