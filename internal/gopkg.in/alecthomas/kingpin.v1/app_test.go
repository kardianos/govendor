package kingpin

import (
	"github.com/stretchr/testify/assert"

	"testing"
	"time"
)

func TestCommander(t *testing.T) {
	c := New("test", "test")
	ping := c.Command("ping", "Ping an IP address.")
	pingTTL := ping.Flag("ttl", "TTL for ICMP packets").Short('t').Default("5s").Duration()

	selected, err := c.Parse([]string{"ping"})
	assert.NoError(t, err)
	assert.Equal(t, "ping", selected)
	assert.Equal(t, 5*time.Second, *pingTTL)

	selected, err = c.Parse([]string{"ping", "--ttl=10s"})
	assert.NoError(t, err)
	assert.Equal(t, "ping", selected)
	assert.Equal(t, 10*time.Second, *pingTTL)
}

func TestRequiredFlags(t *testing.T) {
	c := New("test", "test")
	c.Flag("a", "a").String()
	c.Flag("b", "b").Required().String()

	_, err := c.Parse([]string{"--a=foo"})
	assert.Error(t, err)
	_, err = c.Parse([]string{"--b=foo"})
	assert.NoError(t, err)
}

func TestInvalidDefaultFlagValueErrors(t *testing.T) {
	c := New("test", "test")
	c.Flag("foo", "foo").Default("a").Int()
	_, err := c.Parse([]string{})
	assert.Error(t, err)
}

func TestInvalidDefaultArgValueErrors(t *testing.T) {
	c := New("test", "test")
	cmd := c.Command("cmd", "cmd")
	cmd.Arg("arg", "arg").Default("one").Int()
	_, err := c.Parse([]string{"cmd"})
	assert.Error(t, err)
}

func TestArgsRequiredAfterNonRequiredErrors(t *testing.T) {
	c := New("test", "test")
	cmd := c.Command("cmd", "")
	cmd.Arg("a", "a").String()
	cmd.Arg("b", "b").Required().String()
	_, err := c.Parse([]string{})
	assert.Error(t, err)
}

func TestArgsMultipleRequiredThenNonRequired(t *testing.T) {
	c := New("test", "test")
	cmd := c.Command("cmd", "")
	cmd.Arg("a", "a").Required().String()
	cmd.Arg("b", "b").Required().String()
	cmd.Arg("c", "c").String()
	cmd.Arg("d", "d").String()
	assert.NotPanics(t, func() { c.Parse([]string{}) })
}

func TestDispatchCallbackIsCalled(t *testing.T) {
	dispatched := false
	c := New("test", "")
	c.Command("cmd", "").Dispatch(func(*ParseContext) error {
		dispatched = true
		return nil
	})

	_, err := c.Parse([]string{"cmd"})
	assert.NoError(t, err)
	assert.True(t, dispatched)
}

func TestTopLevelArgWorks(t *testing.T) {
	c := New("test", "test")
	s := c.Arg("arg", "help").String()
	_, err := c.Parse([]string{"foo"})
	assert.NoError(t, err)
	assert.Equal(t, "foo", *s)
}

func TestTopLevelArgCantBeUsedWithCommands(t *testing.T) {
	c := New("test", "test")
	c.Arg("arg", "help").String()
	c.Command("cmd", "help")
	_, err := c.Parse([]string{})
	assert.Error(t, err)
}

func TestTooManyArgs(t *testing.T) {
	a := New("test", "test")
	a.Arg("a", "").String()
	assert.NoError(t, a.init())
	context := Tokenize([]string{"a", "b"})
	_, err := a.parse(context)
	assert.NoError(t, err)
	assert.Equal(t, Tokens{&Token{TokenArg, "b"}}, context.Tokens)
	_, err = a.Parse([]string{"a", "b"})
	assert.Error(t, err)
}

func TestTooManyArgsAfterCommand(t *testing.T) {
	a := New("test", "test")
	a.Command("a", "")
	assert.NoError(t, a.init())
	context := Tokenize([]string{"a", "b"})
	_, err := a.parse(context)
	assert.NoError(t, err)
	assert.Equal(t, Tokens{&Token{TokenArg, "b"}}, context.Tokens)
	_, err = a.Parse([]string{"a", "b"})
	assert.Error(t, err)
}

func TestArgsLooksLikeFlagsWithConsumeRemainder(t *testing.T) {
	a := New("test", "")
	a.Arg("opts", "").Required().Strings()
	_, err := a.Parse([]string{"hello", "-world"})
	assert.Error(t, err)
}
