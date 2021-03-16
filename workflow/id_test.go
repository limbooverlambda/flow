package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"flow/steps"
)

func TestRunIDGen(t *testing.T) {
	s1 := step{
		name:     "s1",
		nextstep: []steps.Step{},
	}
	f := flow{
		name:       "foo",
		startsteps: []steps.Step{s1},
	}

	var context = map[string]interface{}{
		"foo": "bar",
	}
	rid := runID{}
	id, err := rid.Generate("id", f, context)
	assert.NoError(t, err)
	assert.Equal(t, id, "d09e04e1cce2f96ffc951b7d249de5c854902cd8c6f3969435a8993826fa76ab")
}
