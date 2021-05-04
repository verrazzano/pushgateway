// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package verrazzano

import (
	"io"
	"io/ioutil"
	"strings"
	"testing"
)

const (
	typeToken                  = "TYPE"
	standardTypeComment        = "\n# " + typeToken
	replacementTypeToken       = "TYPEX"
	standardReplacementComment = "\n# " + replacementTypeToken
)

func TestAlive(t *testing.T) {
	s := "This is a string that should pass through the filter unmodified."
	uut := NewTypeFilteringReadCloser(ioutil.NopCloser(strings.NewReader(s)))
	b, err := ioutil.ReadAll(uut)
	if err != nil {
		t.Errorf("ReadAll through filter: %v", err)
	}
	if string(b) != s {
		t.Errorf("Expected %s, got %s", s, string(b))
	}
}

func TestFilterSimpleReplace(t *testing.T) {
	tail := "metricName counter\n"
	s := standardTypeComment + " " + tail
	uut := NewTypeFilteringReadCloser(ioutil.NopCloser(strings.NewReader(s)))
	b, err := ioutil.ReadAll(uut)
	if err != nil {
		t.Errorf("ReadAll through filter: %v", err)
	}
	got := string(b)
	expected := standardReplacementComment + tail
	if got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}
}

func TestFilterSimpleFragmentedReplace(t *testing.T) {
	tail := "metricName counter\n"
	fragments := []string{standardTypeComment, " " + tail}
	uut := NewTypeFilteringReadCloser(newFragmentingReadCloser(fragments))
	b, err := ioutil.ReadAll(uut)
	if err != nil {
		t.Errorf("ReadAll through filter: %v", err)
	}
	got := string(b)
	expected := standardReplacementComment + tail
	if got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}
}

func TestFilterNoPrecedingWhiteSpace(t *testing.T) {
	s := "#TYPE Should not change - no whitespace between hash and TYPE"
	doOneString(t, s, s)
}

func TestFilterNoFollowingWhiteSpace(t *testing.T) {
	s := "# TYPE-Should not change - no whitespace after TYPE"
	doOneString(t, s, s)
}

func TestFilterPrecedingMultipleWhiteSpace(t *testing.T) {
	s := "# \t TYPE Should change - multiple whitespace before TYPE"
	expected := "# \t TYPEXShould change - multiple whitespace before TYPE"
	doOneString(t, s, expected)
}

func TestFilterPrecedingMultipleButNoFollowingWhiteSpace(t *testing.T) {
	s := "# \t TYPE-Should not change - no whitespace after TYPE"
	expected := "# \t TYPE-Should not change - no whitespace after TYPE"
	doOneString(t, s, expected)
}

// Try every combination of fragmenting the testString into three small reads.
// The reads are of length 1,1,theRest, 2,1,theRest, N-2,1,1, then 1,2,theRest
// through 1,N-2,1
func TestFilterFragmentationThreeLinesExhaustive(t *testing.T) {
	testString := "# HELP with # mark in it\nPreviousLine{label=\"first\"} 2.178\n# TYPE followingLine counter\n### comment\nfollowingLine{label=\"last\"} 4.669\n"
	filtString := "# HELP with # mark in it\nPreviousLine{label=\"first\"} 2.178\n# TYPEXfollowingLine counter\n### comment\nfollowingLine{label=\"last\"} 4.669\n"

	doFragmentsTest(t, testString, filtString)
}

// This is similar to the previous test but the lines are junk instead of being
// similar to real data
func TestFilterFragmentationIllegalLinesExhaustive(t *testing.T) {
	testString := "HELP this is not TYPE much of anything\n\n  \t\f\n# \t\tTYPE ButThatShouldBeTYPE-\b\b\nTYPE counter SHOULD NOT CHANGE 3.14#\nTYPE\neither"
	filtString := "HELP this is not TYPE much of anything\n\n  \t\f\n# \t\tTYPEXButThatShouldBeTYPE-\b\b\nTYPE counter SHOULD NOT CHANGE 3.14#\nTYPE\neither"

	doFragmentsTest(t, testString, filtString)
}

// This is similar to the previous test but the lines have Unicode
func TestFilterFragmentationWithUnicodeLinesExhaustive(t *testing.T) {
	testString := "HELP \u2101\u2102\u2103\u2104\u2105thing\n\n  \t\f\n# \t\t\u0054YPE ButThatShouldBeTYPE-\b\b\nTYPE counter SHOULD NOT CHANGE 3.14#\nTYPE\neither"
	filtString := "HELP \u2101\u2102\u2103\u2104\u2105thing\n\n  \t\f\n# \t\tTYPEXButThatShouldBeTYPE-\b\b\n\u0054YPE counter SHOULD NOT CHANGE 3.14#\nTYPE\neither"

	doFragmentsTest(t, testString, filtString)
}

func doFragmentsTest(t *testing.T, testString string, filtString string) {
	// It would be fun to generalize this to makeNFragments, etc., but didn't bother
	const FRAGMENT_COUNT = 3
	for i := 0; i < len(testString); i = i + 1 {
		for j := i; j < len(testString); j = j + 1 {
			fragments := makeFragments(testString, i, j) // makes 3 fragments
			uut := NewTypeFilteringReadCloser(newFragmentingReadCloser(fragments))
			buffers := make([][]byte, FRAGMENT_COUNT)
			sizes := make([]int, FRAGMENT_COUNT)
			totalSize := 0
			for k := 0; k < FRAGMENT_COUNT; k = k + 1 {
				buffers[k] = make([]byte, 1024)
				n, err := uut.Read(buffers[k])
				sizes[k] = n
				totalSize += n
				if err != nil {
					if k == FRAGMENT_COUNT-1 && err == io.EOF {
						// Normal EOF
					} else {
						t.Errorf("first read through filter: %d %d %d %d %v", i, j, k, n, err)
						t.FailNow() // else output can be thousands of failures
					}
				}
			}
			// Concatenate all the bytes. We can't use the usual builtin copy,
			// append, etc functions because the buffers are not full and these
			// functions don't take size arguments.
			all := make([]byte, totalSize)
			offset := 0
			for k := 0; k < FRAGMENT_COUNT; k = k + 1 {
				source := buffers[k]
				for n := 0; n < sizes[k]; n += 1 {
					all[offset] = source[n]
					offset += 1
				}
			}
			got := string(all)
			if got != filtString {
				t.Errorf("Expected %s, got %s, i=%d, j=%d", filtString, got, i, j)
				t.FailNow() // else output can be thousands of failures
			}
		}
	}
}

func doOneString(t *testing.T, s string, expected string) {
	fragments := []string{s}
	uut := NewTypeFilteringReadCloser(newFragmentingReadCloser(fragments))
	b, err := ioutil.ReadAll(uut)
	if err != nil {
		t.Errorf("ReadAll through filter: %v", err)
	}
	got := string(b)
	if got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}
}

func makeFragments(s string, i int, j int) []string {
	if i < 1 {
		i = 1
	}
	if j < 1 {
		j = 1
	}
	if i > len(s)-1 {
		i = len(s) - 1
	}
	if j > len(s)-1 {
		j = len(s) - 1
	}
	firstString := s[0:i]
	secondString := s[i:j]
	thirdString := s[j:len(s)]
	return []string{firstString, secondString, thirdString}
}

// The code below is a ReadCloser that accepts a slice of strings and
// returns them on successive calls to Read(). This allows us to write
// deterministic and exhaustive tests that the state-machine based
// recognizer in the filter doesn't fail at buffer boundaries.

type fragmentingReadCloser struct {
	chunks []string
	count  int
	eof    bool
}

func newFragmentingReadCloser(c []string) *fragmentingReadCloser {
	return &fragmentingReadCloser{chunks: c}
}

func (f *fragmentingReadCloser) Read(p []byte) (int, error) {
	result := 0
	err := io.EOF
	if f.eof {
		return result, err
	}
	if f.count < len(f.chunks) {
		f.count += 1
		result = len(f.chunks[f.count-1])
		copy(p, f.chunks[f.count-1])
		err = nil
	}
	if f.count == len(f.chunks) {
		f.eof = true
		err = io.EOF
	}
	return result, err
}

func (f *fragmentingReadCloser) Close() error {
	return nil
}
