package payload

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestExplicitPayloadByteLimit(t *testing.T) {
	for _, scenario := range []struct {
		name, text string
		limit      int
		field      string
	}{
		{name: "below", text: `"a"`, limit: 4},
		{name: "at", text: `"ab"`, limit: 4},
		{name: "above", text: `"abc"`, limit: 4, field: "payload"},
		{name: "zero", limit: 0, field: "payload.max_bytes"},
		{name: "negative", limit: -1, field: "payload.max_bytes"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			err := ValidateJSONWithMaxBytes([]byte(scenario.text), scenario.limit)
			if scenario.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var validation *pkgerrors.ValidationError
			if !errors.As(err, &validation) || validation.Field != scenario.field {
				t.Fatalf("error=%v, want field %s", err, scenario.field)
			}
		})
	}
}

func TestExplicitPayloadLimitKeepsDefaultAndNestingBounds(t *testing.T) {
	large := bytes.Repeat([]byte(" "), MaxBytes+1)
	if err := ValidateJSON(large); err == nil {
		t.Fatal("explicit catalog limits changed the default source limit")
	}
	if err := ValidateJSONWithMaxBytes(large, len(large)); err != nil {
		t.Fatal(err)
	}
	for _, extra := range []int{0, 1} {
		text := strings.Repeat("[", MaxJSONNestingDepth+extra) + "0" + strings.Repeat("]", MaxJSONNestingDepth+extra)
		err := ValidateJSONWithMaxBytes([]byte(text), len(text))
		if (err != nil) != (extra > 0) {
			t.Fatalf("depth=%d error=%v", MaxJSONNestingDepth+extra, err)
		}
	}
}
