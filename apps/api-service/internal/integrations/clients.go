package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client interface {
	ReloadTrafficCache(ctx context.Context) error
	TriggerDestinationHealthcheck(
		ctx context.Context,
		destinationID string,
	) (HealthcheckResult, error)
	TestPostback(
		ctx context.Context,
		payload []byte,
	) error
}

type HealthcheckResult struct {
	DestinationID        string `json:"destination_id"`
	Status               string `json:"status"`
	Error                string `json:"error,omitempty"`
	ProbeStatusCode      int    `json:"probe_status_code,omitempty"`
	ConsecutiveFailures  int    `json:"consecutive_failures"`
	ConsecutiveSuccesses int    `json:"consecutive_successes"`
}

type NoopClient struct{}

func (NoopClient) ReloadTrafficCache(ctx context.Context) error {
	return ctx.Err()
}

func (NoopClient) TriggerDestinationHealthcheck(
	ctx context.Context,
	destinationID string,
) (HealthcheckResult, error) {
	if err := ctx.Err(); err != nil {
		return HealthcheckResult{}, err
	}
	if strings.TrimSpace(destinationID) == "" {
		return HealthcheckResult{}, errors.New("destination id is required")
	}
	return HealthcheckResult{DestinationID: destinationID, Status: "unknown"}, nil
}

func (NoopClient) TestPostback(
	ctx context.Context,
	payload []byte,
) error {
	return ctx.Err()
}

type HTTPClient struct {
	httpClient         httpDoer
	trafficServiceURL  string
	postbackServiceURL string
}

type httpDoer interface {
	Do(req *http.Request) (
		*http.Response,
		error,
	)
}

func NewHTTPClient(
	trafficServiceURL,
	postbackServiceURL string,
) *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		trafficServiceURL: strings.TrimRight(
			trafficServiceURL,
			"/",
		),
		postbackServiceURL: strings.TrimRight(
			postbackServiceURL,
			"/",
		),
	}
}

func NewHTTPClientWithDoer(
	trafficServiceURL,
	postbackServiceURL string,
	doer httpDoer,
) *HTTPClient {
	if doer == nil {
		doer = &http.Client{Timeout: 5 * time.Second}
	}
	return &HTTPClient{
		httpClient: doer,
		trafficServiceURL: strings.TrimRight(
			trafficServiceURL,
			"/",
		),
		postbackServiceURL: strings.TrimRight(
			postbackServiceURL,
			"/",
		),
	}
}

func (c *HTTPClient) ReloadTrafficCache(ctx context.Context) error {
	return c.post(
		ctx,
		c.trafficServiceURL+"/internal/cache/reload",
		nil,
	)
}

func (c *HTTPClient) TriggerDestinationHealthcheck(
	ctx context.Context,
	destinationID string,
) (result HealthcheckResult, resultErr error) {
	destinationID = strings.TrimSpace(destinationID)
	if destinationID == "" {
		return HealthcheckResult{}, errors.New("destination id is required")
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.trafficServiceURL+"/internal/destinations/"+destinationID+"/healthcheck",
		http.NoBody,
	)
	if err != nil {
		return HealthcheckResult{}, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return HealthcheckResult{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, response.Body.Close()) }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return HealthcheckResult{}, errors.New("traffic-service healthcheck failed")
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&result); err != nil {
		return HealthcheckResult{}, err
	}
	return result, nil
}

func (c *HTTPClient) TestPostback(
	ctx context.Context,
	payload []byte,
) error {
	return c.post(
		ctx,
		c.postbackServiceURL+"/internal/postbacks/test",
		payload,
	)
}

func (c *HTTPClient) post(
	ctx context.Context,
	url string,
	body []byte,
) error {
	reader := io.Reader(http.NoBody)
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		reader,
	)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set(
			"Content-Type",
			"application/json",
		)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if closeErr := res.Body.Close(); closeErr != nil {
			return errors.Join(
				errors.New("internal service returned non-2xx status"),
				closeErr,
			)
		}
		return errors.New("internal service returned non-2xx status")
	}
	if closeErr := res.Body.Close(); closeErr != nil {
		return closeErr
	}
	return nil
}
