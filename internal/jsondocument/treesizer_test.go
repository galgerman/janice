package jsondocument

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// decodeForSizer parses JSON the same way the loader does, so the sizer is
// exercised with the value types it actually receives.
func decodeForSizer(t *testing.T, s string) any {
	t.Helper()
	data, err := decodeOrdered(newDecoder(strings.NewReader(s)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCounter(t *testing.T) {
	t.Run("can size object based tree", func(t *testing.T) {
		data := decodeForSizer(t, `{
			"alpha": "abc",
			"bravo": 5,
			"charlie": true,
			"delta": null,
			"echo": [1, 2],
			"foxtrot": {"child": 1}
		}`)
		c := JSONTreeSizer{}
		x, err := c.Calculate(data)
		if assert.NoError(t, err) {
			assert.Equal(t, 10, x)
		}
	})
	t.Run("can size array based tree", func(t *testing.T) {
		data := decodeForSizer(t, `["alpha", "bravo", "charlie", [1, 2], {"child": 1}]`)
		c := JSONTreeSizer{}
		x, err := c.Calculate(data)
		if assert.NoError(t, err) {
			assert.Equal(t, 9, x)
		}
	})
	t.Run("should return error when trying to size invalid structure", func(t *testing.T) {
		c := JSONTreeSizer{}
		_, err := c.Calculate("invalid")
		assert.Error(t, err)
	})
}
