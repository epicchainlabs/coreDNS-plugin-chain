package checkers

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/plugin/pkg/log"
)

type HttpChecker struct {
	logger log.P
	client *http.Client
	port   string
	scheme string
}

type HTTPCheckerParams struct {
	Port    string
	Timeout time.Duration
	Scheme  string
}

const (
	defaultHTTPScheme  = "http"
	defaultHTTPPort    = "80"
	defaultHTTPTimeout = 2 * time.Second
)

func ParseHTTPParams(c *caddy.Controller) (*HTTPCheckerParams, error) {
	prm := &HTTPCheckerParams{}

	for c.NextBlock() {
		key := c.Val()
		args := c.RemainingArgs()
		if len(args) != 1 {
			return nil, fmt.Errorf("'%s' param is expected to have one value, but got '%v'", key, args)
		}
		value := args[0]

		switch key {
		case "port":
			port, err := strconv.Atoi(value)
			if err != nil || port <= 0 {
				return nil, fmt.Errorf("invalid port: '%s'", value)
			}
			prm.Port = value
		case "timeout":
			timeout, err := time.ParseDuration(value)
			if err != nil || timeout <= 0 {
				return nil, fmt.Errorf("invalid timeout '%s'", value)
			}
			prm.Timeout = timeout
		case "scheme":
			if value != "http" && value != "https" {
				return nil, fmt.Errorf("invalid scheme '%s'", value)
			}
			prm.Scheme = value
		default:
			return nil, fmt.Errorf("unknow HTTP parameter: '%s'", c.Val())
		}
	}

	return prm, nil
}

// NewHttpChecker creates http checker.
func NewHttpChecker(logger log.P, prm *HTTPCheckerParams) (*HttpChecker, error) {
	if prm.Timeout <= 0 {
		prm.Timeout = defaultHTTPTimeout
	}

	if len(prm.Port) == 0 {
		prm.Port = defaultHTTPPort
	}

	if len(prm.Scheme) == 0 {
		prm.Scheme = defaultHTTPScheme
	}

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: prm.Timeout,
	}

	return &HttpChecker{
		logger: logger,
		client: client,
		port:   prm.Port,
		scheme: prm.Scheme,
	}, nil
}

func (h HttpChecker) Check(endpoint string) bool {
	response, err := h.client.Get(h.scheme + "://" + net.JoinHostPort(endpoint, h.port))
	if err != nil {
		h.logger.Debugf(err.Error())
		return false
	}
	_ = response.Body.Close()

	return response.StatusCode < http.StatusInternalServerError
}
