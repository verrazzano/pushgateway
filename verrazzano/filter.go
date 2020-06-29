// We need the word "Copyright" here so this file will pass the license check
package verrazzano

import (
	"io"
)

// Filter an underlying stream, recognizing the regex "#[ \t]+TYPE[ \t]" and, when recognized,
// replacing the trailing single whitespace character with an ASCII 'X'.  The data format is here:
// https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md
//
// Understanding this code requires a knowledge of basic properties of the UTF-8 encoding. In
// UTF-8, bytes with a high bit of 0 are always the ASCII characters they appear to be. ASCII
// bytes never occur as the second or succeeding character of a multibyte character. Therefore,
// one can safely scan a UTF-8 encoded byte array seeking specific ASCII bytes.

type ScanState int

const (
	byteHash             = byte('#')
	byteT                = byte('T')
	byteY                = byte('Y')
	byteP                = byte('P')
	byteE                = byte('E')
	byteX                = byte('X')
	byteSpace            = byte(' ')
	byteTab              = byte('\t')
	stateStart ScanState = iota + 1
	stateComment
	stateWhitespace
	stateTSeen
	stateYSeen
	statePSeen
	stateESeen
)

type typeFilteringReadCloser struct {
	wrapped io.ReadCloser
	state   ScanState
}

func NewTypeFilteringReadCloser(rc io.ReadCloser) *typeFilteringReadCloser {
	return &typeFilteringReadCloser{wrapped: rc, state: stateStart}
}

// Implements https://golang.org/pkg/io/#Reader
// Consider passing the logger to tf's constructor for use in assertion failures.
//
// Read and process a buffer. The definition of EOF and the proper handling of
// EOF conditions is peculiar in Golang. See https://golang.org/pkg/io/#Reader
// and note the bit about (0, nil). Golang's own Bufio.Read does 100 retries (!)
// in succession before giving up on (0, nil) and returning io.ErrNoProgress.
func (tf *typeFilteringReadCloser) Read(p []byte) (int, error) {
	n, err := tf.wrapped.Read(p)
	filter(p, n, tf)
	return n, err
}

func (tn *typeFilteringReadCloser) Close() error {
	return tn.wrapped.Close()
}

func filter(p []byte, n int, tf *typeFilteringReadCloser) {
	// When we enter a case in the filter, the byte p[i] lies within
	// the buffer and has not been processed. When we recognize the
	// string "#[ \t]+TYPE[ \t]", we immediately replace the trailing
	// whitespace character with X which ensure the line won't be
	// recognized as a type line.
	//
	// Consider a buffer that ends with the character '#' (HASH). It
	// will be encountered in the first case (stateStart). The index
	// will be set to 1 + the position of the hash, which is outside
	// the buffer. The state will be set to stateComment. If there is no
	// more data, we're good. If there is, the first character of the
	// next buffer will be analyzed starting from state stateComment.
	for i := 0; i < n; i += 1 {
		switch tf.state {
		case stateStart:
			hash := indexByteWithOffset(p, i, n, byteHash)
			if hash == -1 {
				return // leaving tf.state == stateStart
			}
			i = hash
			tf.state = stateComment
		case stateComment:
			if isAsciiSpaceOrTab(p[i]) {
				tf.state = stateWhitespace
			} else {
				tf.state = stateStart
			}
		case stateWhitespace:
			// It's tempting to optimize by looping past repeating
			// white space, but this can cause problems at buffer
			// boundaries. And it's rare anyway, because most TYPE
			// comments are written by the Prometheus client libs,
			// which always put exactly one space after the hash.
			if isAsciiSpaceOrTab(p[i]) {
				// multiple whitespace is allowed between # and T.
				tf.state = stateWhitespace
			} else if p[i] == byteT {
				tf.state = stateTSeen
			} else {
				tf.state = stateStart
			}
		case stateTSeen:
			if p[i] == byteY {
				tf.state = stateYSeen
			} else {
				tf.state = stateStart
			}
		case stateYSeen:
			if p[i] == byteP {
				tf.state = statePSeen
			} else {
				tf.state = stateStart
			}
		case statePSeen:
			if p[i] == byteE {
				tf.state = stateESeen
			} else {
				tf.state = stateStart
			}
		case stateESeen:
			if isAsciiSpaceOrTab(p[i]) {
				p[i] = byteX
			}
			tf.state = stateStart
			// Again, no reason to optimize (consume rest of line,
			// etc.) because the START state will do a fast-as-possible
			// scan for the next hash character in the buffer anyway.
		default:
			// internal error, log here
			tf.state = stateStart
		}
	}
}

// We do not use Golang's bytes.Index...() functions because they are expensive to
// apply repeatedly to a large slice. In order to step through a large slice using
// them, it's necessary to re-slice the array after every match. This allocates a
// small structure each time. This is ridiculous if all you want to do is find a
// byte. With this function we are old-school.
func indexByteWithOffset(p []byte, offset int, n int, b byte) int {
	if offset < 0 || n > len(p) {
		return -1
	}
	for i := offset; i < n; i += 1 {
		if p[i] == b {
			return i
		}
	}
	return -1
}

func isAsciiSpaceOrTab(b byte) bool {
	return b == byteSpace || b == byteTab
}
