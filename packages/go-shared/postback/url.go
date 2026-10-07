package postback

import (
	"errors"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var token = regexp.MustCompile(`\{([^{}]+)\}`)

var allowedMacros = map[string]bool{
	"click_id": true, "cid": true, "source_click_id": true,
	"conversion_id": true, "transaction_id": true, "event_type": true,
	"status": true, "currency": true, "network_id": true,
	"payout": true, "sum": true, "revenue": true,
	"sub1": true, "sub2": true, "sub3": true, "sub4": true,
	"sub5": true, "sub6": true, "sub7": true, "sub8": true,
	"sub9": true, "sub10": true, "s1": true, "s2": true, "s3": true, "s4": true,
}

func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") ||
		u.Hostname() == "" || u.User != nil || u.Fragment != "" || len(raw) > 4096 {
		return errors.New("postback URL must be an HTTP(S) URL without credentials or fragment")
	}
	if strings.ContainsAny(
		u.Host+u.Path,
		"{}",
	) {
		return errors.New("postback macros are allowed only in query values")
	}
	if address, err := netip.ParseAddr(u.Hostname()); err == nil && !PublicAddress(address) {
		return errors.New("postback URL must use a public address")
	}
	if strings.EqualFold(
		u.Hostname(),
		"localhost",
	) {
		return errors.New("postback URL must use a public host")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return errors.New("invalid postback query")
	}
	for key, values := range query {
		if strings.ContainsAny(
			key,
			"{}",
		) {
			return errors.New("query parameter names cannot contain macros")
		}
		for _, value := range values {
			for _, match := range token.FindAllStringSubmatch(
				value,
				-1,
			) {
				if !allowedMacros[strings.ToLower(match[1])] {
					return errors.New("unknown postback macro: " + match[1])
				}
			}
			if strings.ContainsAny(
				token.ReplaceAllString(
					value,
					"",
				),
				"{}",
			) {
				return errors.New("invalid postback macro")
			}
		}
	}
	return nil
}

func Render(
	raw string,
	values map[string]string,
) (
	string,
	error,
) {
	if err := ValidateURL(raw); err != nil {
		return "", err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	query := u.Query()
	for key, items := range query {
		for i, value := range items {
			items[i] = token.ReplaceAllStringFunc(
				value,
				func(name string) string { return values[strings.ToLower(name[1:len(name)-1])] },
			)
		}
		query[key] = items
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func UsesSourceClickID(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	for _, values := range u.Query() {
		for _, value := range values {
			if strings.Contains(
				strings.ToLower(value),
				"{source_click_id}",
			) {
				return true
			}
		}
	}
	return false
}

func PublicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	// Shared address space, metadata, documentation and benchmark networks are not public receivers.
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(raw).Contains(address) {
			return false
		}
	}
	return true
}

func Payout(value float64) string {
	return strconv.FormatFloat(
		value,
		'f',
		-1,
		64,
	)
}
