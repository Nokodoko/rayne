package keys

import (
	"fmt"
	"os"
)

var apiKeyName string = "DD_API_KEY"
var appKeyName string = "DD_APP_KEY"

// DefaultBaseURL is the default Datadog API endpoint (US Government)
const DefaultBaseURL = "https://api.ddog-gov.com"

func Api() string {
	_, ok := os.LookupEnv(apiKeyName)
	switch {
	case !ok:
		return fmt.Sprintf("%s environment variable not set, or is set to a different name.", apiKeyName)
	default:
		key := os.Getenv(apiKeyName)
		return key
	}
}

func App() string {
	_, ok := os.LookupEnv(appKeyName)
	switch {
	case !ok:
		return fmt.Sprintf("%s environment variable not set, or is set to a different name.", appKeyName)
	default:
		key := os.Getenv(appKeyName)
		return key
	}
}

// Credentials holds Datadog API authentication information
type Credentials struct {
	APIKey  string
	AppKey  string
	BaseURL string
}

// Default returns credentials from environment variables
func Default() Credentials {
	return Credentials{
		APIKey:  Api(),
		AppKey:  App(),
		BaseURL: DefaultBaseURL,
	}
}

// BuildURL constructs a full API URL from the credentials' base URL
func (c Credentials) BuildURL(path string) string {
	return c.BaseURL + path
}
