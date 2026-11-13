package table

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	var out strings.Builder

	err := Render(&out, []string{"URI", "Name"}, [][]string{
		{"api/users", "users.index"},
		{"/"}, // short row: padded with an empty cell
		{"café", ""},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `+-----------+-------------+
| URI       | Name        |
+-----------+-------------+
| api/users | users.index |
| /         |             |
| café      |             |
+-----------+-------------+
`
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}
