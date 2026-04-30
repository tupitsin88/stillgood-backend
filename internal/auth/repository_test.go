package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserSearchPattern(t *testing.T) {
	assert.Equal(t, "", userSearchPattern("   "))
	assert.Equal(t, "%alice@example.local%", userSearchPattern(" alice@example.local "))
	assert.Equal(t, `%100\%\_ready\\now%`, userSearchPattern(`100%_ready\now`))
}
