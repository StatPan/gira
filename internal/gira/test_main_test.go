package gira

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Setenv("GIRA_DEV_PR_GRAPHQL_FALLBACK", "1")
	os.Exit(m.Run())
}
