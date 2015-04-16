package kingpin

import (
	"github.com/stretchr/testify/assert"

	"testing"
)

func TestNestedCommands(t *testing.T) {
	app := New("app", "")
	sub1 := app.Command("sub1", "")
	sub1.Flag("sub1", "")
	subsub1 := sub1.Command("sub1sub1", "")
	subsub1.Command("sub1sub1end", "")

	sub2 := app.Command("sub2", "")
	sub2.Flag("sub2", "")
	sub2.Command("sub2sub1", "")

	context := Tokenize([]string{"sub1", "sub1sub1", "sub1sub1end"})
	selected, err := app.parse(context)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(context.Tokens))
	assert.Equal(t, "sub1 sub1sub1 sub1sub1end", selected)
}

func TestNestedCommandsWithArgs(t *testing.T) {
	app := New("app", "")
	cmd := app.Command("a", "").Command("b", "")
	a := cmd.Arg("a", "").String()
	b := cmd.Arg("b", "").String()
	context := Tokenize([]string{"a", "b", "c", "d"})
	selected, err := app.parse(context)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(context.Tokens))
	assert.Equal(t, "a b", selected)
	assert.Equal(t, "c", *a)
	assert.Equal(t, "d", *b)
}

func TestNestedCommandsWithFlags(t *testing.T) {
	app := New("app", "")
	cmd := app.Command("a", "").Command("b", "")
	a := cmd.Flag("aaa", "").Short('a').String()
	b := cmd.Flag("bbb", "").Short('b').String()
	err := app.init()
	assert.NoError(t, err)
	context := Tokenize([]string{"a", "b", "--aaa", "x", "-b", "x"})
	selected, err := app.parse(context)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(context.Tokens))
	assert.Equal(t, "a b", selected)
	assert.Equal(t, "x", *a)
	assert.Equal(t, "x", *b)
}
