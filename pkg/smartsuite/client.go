package smartsuite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/SmartSuiteFoundry/scim-mediator/pkg/models"
)

// Client is a client for interacting with the SmartSuite SCIM API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new SmartSuite API client.
func NewClient(baseURL, apiKey string) (*Client, error) {
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("BaseURL and APIKey must be provided")
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}, nil
}

// --- Public Methods for Users and Groups ---

// GetUserByUsername fetches a single user by their exact userName using a filter.
// It returns (nil, nil) if the user is not found.
func (c *Client) GetUserByUsername(ctx context.Context, username string) (*models.SCIMUser, error) {
	endpointURL, _ := url.Parse(fmt.Sprintf("%s/Users", c.BaseURL))
	queryParams := url.Values{}
	// Note: URL encoding for the filter value is handled by RawQuery
	queryParams.Set("filter", fmt.Sprintf(`userName eq "%s"`, username))
	endpointURL.RawQuery = queryParams.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", endpointURL.String(), nil)
	if err != nil {
		return nil, err
	}

	body, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}

	var listResponse models.ListResponse
	if err := json.Unmarshal(body, &listResponse); err != nil {
		return nil, fmt.Errorf("error unmarshaling user filter response: %w", err)
	}

	if listResponse.TotalResults == 0 || len(listResponse.Resources) == 0 {
		return nil, nil // User not found
	}

	var user models.SCIMUser
	resourceBytes, _ := json.Marshal(listResponse.Resources[0])
	if err := json.Unmarshal(resourceBytes, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal found user: %w", err)
	}

	return &user, nil
}

// GetUsers fetches all users from the SCIM API, handling pagination.
func (c *Client) GetUsers(ctx context.Context) ([]models.SCIMUser, error) {
	var allUsers []models.SCIMUser
	startIndex := 1
	itemsPerPage := 100

	for {
		endpointURL, _ := url.Parse(fmt.Sprintf("%s/Users", c.BaseURL))
		queryParams := url.Values{}
		queryParams.Set("startIndex", strconv.Itoa(startIndex))
		queryParams.Set("count", strconv.Itoa(itemsPerPage))
		endpointURL.RawQuery = queryParams.Encode()

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL.String(), nil)
		if err != nil {
			return nil, err
		}

		body, err := c.doRequestWithRetry(ctx, req)
		if err != nil {
			return nil, err
		}

		var listResponse models.ListResponse
		if err := json.Unmarshal(body, &listResponse); err != nil {
			return nil, fmt.Errorf("error unmarshaling user list response: %w", err)
		}

		if len(listResponse.Resources) == 0 {
			break
		}

		for _, resource := range listResponse.Resources {
			var user models.SCIMUser
			resourceBytes, _ := json.Marshal(resource)
			if err := json.Unmarshal(resourceBytes, &user); err == nil {
				allUsers = append(allUsers, user)
			}
		}

		if len(allUsers) >= listResponse.TotalResults {
			break
		}
		startIndex += len(listResponse.Resources)
	}
	return allUsers, nil
}

// GetGroups fetches all groups from the SCIM API, handling pagination.
func (c *Client) GetGroups(ctx context.Context) ([]models.SCIMGroup, error) {
	var allGroups []models.SCIMGroup
	startIndex := 1
	itemsPerPage := 100

	for {
		endpointURL, _ := url.Parse(fmt.Sprintf("%s/Groups", c.BaseURL))
		queryParams := url.Values{}
		queryParams.Set("startIndex", strconv.Itoa(startIndex))
		queryParams.Set("count", strconv.Itoa(itemsPerPage))
		endpointURL.RawQuery = queryParams.Encode()

		req, err := http.NewRequestWithContext(ctx, "GET", endpointURL.String(), nil)
		if err != nil {
			return nil, err
		}

		body, err := c.doRequestWithRetry(ctx, req)
		if err != nil {
			return nil, err
		}

		var listResponse models.ListResponse
		if err := json.Unmarshal(body, &listResponse); err != nil {
			return nil, fmt.Errorf("error unmarshaling group list response: %w", err)
		}

		if len(listResponse.Resources) == 0 {
			break
		}

		for _, resource := range listResponse.Resources {
			var group models.SCIMGroup
			resourceBytes, _ := json.Marshal(resource)
			if err := json.Unmarshal(resourceBytes, &group); err == nil {
				allGroups = append(allGroups, group)
			}
		}

		if len(allGroups) >= listResponse.TotalResults {
			break
		}
		startIndex += len(listResponse.Resources)
	}
	return allGroups, nil
}

// CreateUser sends a POST request to create a new user.
func (c *Client) CreateUser(ctx context.Context, user models.SCIMUser) (*models.SCIMUser, error) {
	user.Schemas = []string{"urn:ietf:params:scim:schemas:core:2.0:User", "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"}
	payload, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create user payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/Users", c.BaseURL), bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	body, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	var createdUser models.SCIMUser
	if err := json.Unmarshal(body, &createdUser); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created user response: %w", err)
	}
	return &createdUser, nil
}

// DeleteUser sends a DELETE request to permanently remove a user.
func (c *Client) DeleteUser(ctx context.Context, scimID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/Users/%s", c.BaseURL, scimID), nil)
	if err != nil {
		return err
	}
	_, err = c.doRequestWithRetry(ctx, req)
	return err
}

// PatchUser sends a PATCH request to update a user's attributes.
func (c *Client) PatchUser(ctx context.Context, scimID string, operations []models.SCIMPatchOp) error {
	payload := map[string]interface{}{
		"schemas":    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": operations,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal patch payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "PATCH", fmt.Sprintf("%s/Users/%s", c.BaseURL, scimID), bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	_, err = c.doRequestWithRetry(ctx, req)
	return err
}

// CreateGroup sends a POST request to create a new group.
func (c *Client) CreateGroup(ctx context.Context, group models.SCIMGroup) (*models.SCIMGroup, error) {
	payload := map[string]interface{}{
		"schemas":     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		"displayName": group.DisplayName,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create group payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/Groups", c.BaseURL), bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}
	body, err := c.doRequestWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	var createdGroup models.SCIMGroup
	if err := json.Unmarshal(body, &createdGroup); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created group response: %w", err)
	}
	return &createdGroup, nil
}

// PatchGroup sends a PATCH request to modify a group's members.
func (c *Client) PatchGroup(ctx context.Context, scimID string, operations []models.SCIMPatchOp) error {
	payload := map[string]interface{}{
		"schemas":    []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"},
		"Operations": operations,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal patch group payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "PATCH", fmt.Sprintf("%s/Groups/%s", c.BaseURL, scimID), bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	_, err = c.doRequestWithRetry(ctx, req)
	return err
}

// --- Private Helper for HTTP Requests with Retry Logic ---

func (c *Client) doRequestWithRetry(ctx context.Context, req *http.Request) ([]byte, error) {
	var lastErr error
	maxRetries := 4
	baseBackoff := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		var reqBodyBytes []byte
		if req.Body != nil {
			reqBodyBytes, _ = io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
		}

		cloneReq := req.Clone(ctx)
		if req.Body != nil {
			cloneReq.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
		}

		cloneReq.Header.Set("Authorization", "Bearer "+c.APIKey)
		cloneReq.Header.Set("Content-Type", "application/scim+json")
		cloneReq.Header.Set("Accept", "application/scim+json")

		slog.Debug("Making API request", "method", cloneReq.Method, "url", cloneReq.URL.String())

		res, httpErr := c.HTTPClient.Do(cloneReq)
		if httpErr != nil {
			lastErr = httpErr
			slog.Warn("HTTP transport error, will retry...", "attempt", attempt+1, "max_attempts", maxRetries, "error", lastErr)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
			backoff := float64(baseBackoff) * math.Pow(2, float64(attempt))
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			sleepDuration := time.Duration(backoff) + jitter

			slog.Warn("API returned retryable error, backing off...", "status_code", res.StatusCode, "attempt", attempt+1, "max_attempts", maxRetries, "sleep_duration", sleepDuration)
			res.Body.Close()
			time.Sleep(sleepDuration)
			lastErr = fmt.Errorf("API returned status %d", res.StatusCode)
			continue
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if res.StatusCode == http.StatusNoContent {
			return nil, nil
		}

		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, fmt.Errorf("api request failed with non-retryable status %d: %s", res.StatusCode, string(body))
		}

		return body, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
}
