package integrations

import (
	"bytes"
	"context"
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
	) error
	TestPostback(
		ctx context.Context,
		payload []byte,
	) error
}

type NoopClient struct{}

func (NoopClient) ReloadTrafficCache(ctx context.Context) error {
	return ctx.Err()
}

func (NoopClient) TriggerDestinationHealthcheck(
	ctx context.Context,
	destinationID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(destinationID) == "" {
		return errors.New("destination id is required")
	}
	return nil
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
) error {
	destinationID = strings.TrimSpace(destinationID)
	if destinationID == "" {
		return errors.New("destination id is required")
	}
	return c.post(
		ctx,
		c.trafficServiceURL+"/internal/destinations/"+destinationID+"/healthcheck",
		nil,
	)
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
