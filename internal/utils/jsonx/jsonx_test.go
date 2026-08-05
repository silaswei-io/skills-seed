package jsonx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalFromTextStrictKeepsValidJSONBeforeRepair(t *testing.T) {
	input := `{
  "focuses": [
    {
      "focus_id": "nginx-config-formatting",
      "profile_refresh_recommended": {
        "needed": false,
        "reason": "valid nested object must stay nested"
      },
      "good_example": "func x() {\n\tif k == '{' {\n\t\treturn\n\t}\n}"
    }
  ]
}`

	var out struct {
		Focuses []struct {
			FocusID                   string `json:"focus_id"`
			ProfileRefreshRecommended struct {
				Needed bool   `json:"needed"`
				Reason string `json:"reason"`
			} `json:"profile_refresh_recommended"`
			GoodExample string `json:"good_example"`
		} `json:"focuses"`
	}

	require.NoError(t, UnmarshalFromTextStrict(input, &out))
	require.Len(t, out.Focuses, 1)
	require.Equal(t, "nginx-config-formatting", out.Focuses[0].FocusID)
	require.False(t, out.Focuses[0].ProfileRefreshRecommended.Needed)
}
