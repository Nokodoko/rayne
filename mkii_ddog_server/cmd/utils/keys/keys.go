package keys

import (
	"fmt"
	"os"
)

var apiKeyName string = "DD_API_KEY"
var appKeyName string = "DD_APP_KEY"

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
