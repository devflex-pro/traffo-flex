package outbound

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/postback"
)

func NewPublicClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Dial the validated IP directly, so a second DNS resolution cannot bypass the check.
	transport.Proxy = nil
	transport.DialContext = publicDial
	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
		CheckRedirect: func(
			r *http.Request,
			via []*http.Request,
		) error {
			return http.ErrUseLastResponse
		},
	}
}

func publicDial(
	ctx context.Context,
	network string,
	address string,
) (
	net.Conn,
	error,
) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupNetIP(
		ctx,
		"ip",
		host,
	)
	if err != nil {
		return nil, errors.New("postback DNS lookup failed")
	}
	if len(addresses) == 0 {
		return nil, errors.New("postback host has no addresses")
	}
	for _, ip := range addresses {
		if !postback.PublicAddress(ip) {
			return nil, errors.New("postback host resolves to a private or reserved address")
		}
	}
	var dialErr error
	for _, ip := range addresses {
		connection, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(
			ctx,
			network,
			net.JoinHostPort(
				ip.String(),
				port,
			),
		)
		if err == nil {
			return connection, nil
		}
		dialErr = err
	}
	return nil, dialErr
}

func Send(
	ctx context.Context,
	client *http.Client,
	target string,
	idempotencyKey string,
) (
	int,
	error,
) {
	if err := postback.ValidateURL(target); err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		target,
		nil,
	)
	if err != nil {
		return 0, errors.New("invalid postback request")
	}
	request.Header.Set(
		"Idempotency-Key",
		idempotencyKey,
	)
	request.Header.Set(
		"User-Agent",
		"TraffoFlex-Postback/1.0",
	)
	response, err := client.Do(request)
	if err != nil {
		return 0, errors.New("postback request failed")
	}
	_, readErr := io.Copy(
		io.Discard,
		io.LimitReader(
			response.Body,
			4096,
		),
	)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		return response.StatusCode, errors.New("postback response read failed")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, errors.New("postback receiver returned a non-2xx status")
	}
	return response.StatusCode, nil
}
