package handlers

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLotCursor_RoundTrip(t *testing.T) {
	c := lotCursor{OnlineRank: 0, GearScore: 9.75, PricePerGB: 0.05, ID: "abc-123"}
	tok := encodeCursor(c)
	require.NotEmpty(t, tok)

	got, ok := decodeCursor(tok)
	require.True(t, ok)
	assert.Equal(t, c, got)
}

func TestDecodeCursor_RejectsGarbage(t *testing.T) {
	_, ok := decodeCursor("not-base64!!!")
	assert.False(t, ok)

	_, ok = decodeCursor("")
	assert.False(t, ok)
}

// TestMarketplaceListLots_CursorPaginationInSource guards Fix #7: the handler
// must use keyset pagination, not bare LIMIT without cursor support.
func TestMarketplaceListLots_CursorPaginationInSource(t *testing.T) {
	src, err := os.ReadFile("marketplace.go")
	require.NoError(t, err)
	text := string(src)

	for _, marker := range []string{
		"next_cursor",
		"decodeCursor",
		"encodeCursor",
		"hasCursor",
		"-wl.gear_score",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("Fix #7 regression: marketplace.go missing cursor pagination marker %q", marker)
		}
	}
}
