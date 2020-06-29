// We need the word "Copyright" here so this file will pass the license check
package verrazzano

import (
	"io/ioutil"
	"strconv"
	"strings"
	"testing"
)

func TestDefinerAlive(t *testing.T) {
	s := "This is a string that should pass through the filter unmodified."
	expected := TypeDefinitions + s
	uut := NewTypeDefiningReadCloser(NewTypeFilteringReadCloser(ioutil.NopCloser(strings.NewReader(s))))
	b, err := ioutil.ReadAll(uut)
	if err != nil {
		t.Errorf("ReadAll through filter: %v", err)
	}
	got := string(b)
	if got != expected {
		t.Errorf("Expected %s (len %d), got %s (len %d)", strconv.Quote(expected), len(expected), strconv.Quote(got), len(got))
	}
}

func TestDefinerNoPrecedingWhiteSpace(t *testing.T) {
	s := "#TYPE Should not change - no whitespace between hash and TYPE"
	doOneStringDefiner(t, s, TypeDefinitions+s)
}

func TestDefinerNoFollowingWhiteSpace(t *testing.T) {
	s := "# TYPE-Should not change - no whitespace after TYPE"
	doOneStringDefiner(t, s, TypeDefinitions+s)
}

func TestDefinerPrecedingMultipleWhiteSpace(t *testing.T) {
	s := "# \t TYPE Should change - multiple whitespace before TYPE"
	expected := "# \t TYPEXShould change - multiple whitespace before TYPE"
	doOneStringDefiner(t, s, TypeDefinitions+expected)
}

func TestDefinerPrecedingMultipleButNoFollowingWhiteSpace(t *testing.T) {
	s := "# \t TYPE-Should not change - no whitespace after TYPE"
	expected := "# \t TYPE-Should not change - no whitespace after TYPE"
	doOneStringDefiner(t, s, TypeDefinitions+expected)
}

func TestDefinerThreeLines(t *testing.T) {
	testString := "# HELP with # mark in it\nPreviousLine{label=\"first\"} 2.178\n# TYPE followingLine counter\n### comment\nfollowingLine{label=\"last\"} 4.669\n"
	filtString := "# HELP with # mark in it\nPreviousLine{label=\"first\"} 2.178\n# TYPEXfollowingLine counter\n### comment\nfollowingLine{label=\"last\"} 4.669\n"

	doOneStringDefiner(t, testString, TypeDefinitions+filtString)
}

// This is similar to the previous test but the lines are junk instead of being
// similar to real data
func TestDefinerIllegalLines(t *testing.T) {
	testString := "HELP this is not TYPE much of anything\n\n  \t\f\n# \t\tTYPE ButThatShouldBeTYPE-\b\b\nTYPE counter SHOULD NOT CHANGE 3.14#\nTYPE\neither"
	filtString := "HELP this is not TYPE much of anything\n\n  \t\f\n# \t\tTYPEXButThatShouldBeTYPE-\b\b\nTYPE counter SHOULD NOT CHANGE 3.14#\nTYPE\neither"

	doOneStringDefiner(t, testString, TypeDefinitions+filtString)
}

// This is similar to the previous test but the lines have Unicode
func TestDefinerWithUnicodeLines(t *testing.T) {
	testString := "HELP \u2101\u2102\u2103\u2104\u2105thing\n\n  \t\f\n# \t\t\u0054YPE ButThatShouldBeTYPE-\b\b\nTYPE counter SHOULD NOT CHANGE 3.14#\nTYPE\neither"
	filtString := "HELP \u2101\u2102\u2103\u2104\u2105thing\n\n  \t\f\n# \t\tTYPEXButThatShouldBeTYPE-\b\b\n\u0054YPE counter SHOULD NOT CHANGE 3.14#\nTYPE\neither"

	doOneStringDefiner(t, testString, TypeDefinitions+filtString)
}

func doOneStringDefiner(t *testing.T, s string, expected string) {
	fragments := []string{s}
	uut := NewTypeDefiningReadCloser(NewTypeFilteringReadCloser(newFragmentingReadCloser(fragments)))
	b, err := ioutil.ReadAll(uut)
	if err != nil {
		t.Errorf("ReadAll through filter: %v", err)
	}
	got := string(b)
	if got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}
}
