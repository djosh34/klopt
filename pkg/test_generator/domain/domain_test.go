package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDomainContextParseRejectsMissingType(t *testing.T) {
	node := json.RawMessage(`{"nullable":true}`)
	dc := DomainContext{}

	domain, err := dc.Parse(&node)
	require.Error(t, err)
	require.Nil(t, domain)
	require.Empty(t, dc.domainStore)
}

func TestDomainContextParseRejectsUnknownType(t *testing.T) {
	node := json.RawMessage(`{"type":"unknown"}`)
	dc := DomainContext{}

	domain, err := dc.Parse(&node)
	require.Error(t, err)
	require.Nil(t, domain)
	require.Empty(t, dc.domainStore)
}

func TestDomainContextParseRejectsMixedTypeArray(t *testing.T) {
	node := json.RawMessage(`{"type":["string","integer"]}`)
	dc := DomainContext{}

	domain, err := dc.Parse(&node)
	require.Error(t, err)
	require.Nil(t, domain)
	require.Empty(t, dc.domainStore)
}
