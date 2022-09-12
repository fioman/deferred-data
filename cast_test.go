package deferred

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type User struct {
	Name string `json:"name"`
}

func TestCastJson(t *testing.T) {
	user, err := CastJson[User]("{\"name\": \"jack\"}", nil)
	assert.Nil(t, err)
	assert.Equal(t, "jack", user.Name)
}
