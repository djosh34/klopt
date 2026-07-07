package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDomainContextParseUsesDefaultParser(t *testing.T) {
	node := json.RawMessage(`{"type":"unknown"}`)
	dc := DomainContext{}

	domain, err := dc.Parse(&node)
	require.NoError(t, err)
	require.Nil(t, domain)
	require.NotNil(t, dc.domainStore)
	require.Contains(t, dc.domainStore, domain)
}
