package httpclient

import (
	"net"
	"net/http"
	"time"

	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
)

// DefaultClient is a shared HTTP client for general API requests.
// Uses connection pooling for efficient resource usage.
var DefaultClient = httptrace.WrapClient(&http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxConnsPerHost:     10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
})

// AgentClient is a shared HTTP client for long-running agent requests.
// Has extended timeout for AI analysis operations.
var AgentClient = httptrace.WrapClient(&http.Client{
	Timeout: 180 * time.Second, // 3 minutes for Claude analysis
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxConnsPerHost:     5,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
})

// NotifyClient is a shared HTTP client for quick notification requests.
// Short timeout since notifications should be fast.
var NotifyClient = httptrace.WrapClient(&http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        50,
		MaxConnsPerHost:     10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     60 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
})

// ForwardingClient is a shared HTTP client for webhook forwarding.
// Moderate timeout for external webhook endpoints.
var ForwardingClient = httptrace.WrapClient(&http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        50,
		MaxConnsPerHost:     5,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     60 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
})

// DatadogClient is a shared HTTP client for Datadog API requests.
// Moderate timeout for API operations.
var DatadogClient = httptrace.WrapClient(&http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        50,
		MaxConnsPerHost:     10,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
})
