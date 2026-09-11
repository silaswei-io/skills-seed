package jsonx

import (
	"encoding/json"
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

func TestUnmarshalStrictRejectsUnknownAndTrailingValues(t *testing.T) {
	var value struct {
		Name string `json:"name"`
	}
	require.ErrorContains(t, UnmarshalStrict([]byte(`{"name":"demo","extra":true}`), &value), "unknown field")
	require.Error(t, UnmarshalStrict([]byte(`{"name":"demo"} {"name":"later"}`), &value))
	require.Error(t, UnmarshalStrict([]byte(`{"name":`), &value))
}

func TestUnmarshalFromTextStrictUsesExtractedAndNestedCandidates(t *testing.T) {
	type result struct {
		Name string `json:"name"`
	}
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "narrative", text: "before ```json\n{\"name\":\"demo\"}\n``` after", want: "demo"},
		{name: "structured output", text: `{"structured_output":{"name":"nested"}}`, want: "nested"},
		{name: "result string", text: `{"result":"prefix {\"name\":\"wrapped\"} suffix"}`, want: "wrapped"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got result
			require.NoError(t, UnmarshalFromTextStrict(test.text, &got))
			require.Equal(t, test.want, got.Name)
		})
	}

	var got result
	require.ErrorIs(t, UnmarshalFromTextStrict("plain text", &got), ErrNoJSONCandidate)
	require.Error(t, UnmarshalFromTextStrict(`{"structured_output":{"unknown":true}}`, &got))
}

func TestCandidatesRepairsAndDeduplicatesJSON(t *testing.T) {
	candidates := Candidates("prefix {name: 'demo',} suffix")
	require.NotEmpty(t, candidates)
	var value map[string]string
	require.NoError(t, json.Unmarshal([]byte(candidates[0]), &value))
	require.Equal(t, "demo", value["name"])
	require.Equal(t, []string{"first", "second"}, appendUnique([]string{"first"}, " second "))
	require.Equal(t, []string{"first"}, appendUnique([]string{"first"}, "first"))
	require.Equal(t, []string{"first"}, appendUnique([]string{"first"}, " "))
}

func TestRepairCandidate(t *testing.T) {
	repaired, ok := RepairCandidate(`  {"name":"demo"}  `)
	require.True(t, ok)
	require.JSONEq(t, `{"name":"demo"}`, repaired)

	repaired, ok = RepairCandidate("{" + "name: 'demo',}")
	require.True(t, ok)
	require.JSONEq(t, `{"name":"demo"}`, repaired)

	for _, input := range []string{"", "plain text", "null", "42"} {
		_, ok = RepairCandidate(input)
		require.False(t, ok, input)
	}
}

func TestFormatIfJSON(t *testing.T) {
	formatted, ok := FormatIfJSON(`{"name":"demo","count":2}`)
	require.True(t, ok)
	require.Equal(t, "{\n  \"count\": 2,\n  \"name\": \"demo\"\n}", formatted)

	for _, input := range []string{"", "not json"} {
		formatted, ok = FormatIfJSON(input)
		require.False(t, ok)
		require.Empty(t, formatted)
	}
}

func TestNestedOutputCandidatesRejectsMalformedEnvelope(t *testing.T) {
	require.Nil(t, nestedOutputCandidates("{"))
	require.Empty(t, nestedOutputCandidates(`{"structured_output":null,"result":42}`))
}
