// We need the word "Copyright" here so this file will pass the license check
package verrazzano

import (
	"bytes"
	"io"
)

// This class is typically wrapped around a TypeFilteringReadCloser, which
// removes (in a trixie way) all the type information from inbound posts.
// This causes all inbound metrics to assume the default type "untyped",
// preventing conflicts between metrics pushed here by different pushers.
//
// But that's not quite adequate to completely prevent type conflicts. It
// turns out the Pushgateway defines a small set of "self-" metrics about
// its own state and stores them with the pushed metrics. So conflicts can
// occur between the "untyped" metrics pushed by clients and the self-
// metrics defined internally by the pushgateway.
//
// It turns out that the Pushgateway allows type definitions that don't
// correspond to any metric name. So we can work around the "self-metric
// conflict" issue by statically predefining the type of every self-metric
// name at the top of every inbound push. Since the body of the push passes
// through the TypeFilteringReadCloser we wrap, it never contains any other
// type information that might conflict with these definitions.
//
// So this class is instanced once for each inbound post and prepends a
// "header" made up of the fixed string below, after which this class
// becomes a pass-through for the rest of the stream.

const TypeDefinitions = `# TYPE go_gc_duration_seconds summary
# TYPE go_goroutines gauge
# TYPE go_info gauge
# TYPE go_memstats_alloc_bytes gauge
# TYPE go_memstats_alloc_bytes_total counter
# TYPE go_memstats_buck_hash_sys_bytes gauge
# TYPE go_memstats_frees_total counter
# TYPE go_memstats_gc_cpu_fraction gauge
# TYPE go_memstats_gc_sys_bytes gauge
# TYPE go_memstats_heap_alloc_bytes gauge
# TYPE go_memstats_heap_idle_bytes gauge
# TYPE go_memstats_heap_inuse_bytes gauge
# TYPE go_memstats_heap_objects gauge
# TYPE go_memstats_heap_released_bytes gauge
# TYPE go_memstats_heap_sys_bytes gauge
# TYPE go_memstats_last_gc_time_seconds gauge
# TYPE go_memstats_lookups_total counter
# TYPE go_memstats_mallocs_total counter
# TYPE go_memstats_mcache_inuse_bytes gauge
# TYPE go_memstats_mcache_sys_bytes gauge
# TYPE go_memstats_mspan_inuse_bytes gauge
# TYPE go_memstats_mspan_sys_bytes gauge
# TYPE go_memstats_next_gc_bytes gauge
# TYPE go_memstats_other_sys_bytes gauge
# TYPE go_memstats_stack_inuse_bytes gauge
# TYPE go_memstats_stack_sys_bytes gauge
# TYPE go_memstats_sys_bytes gauge
# TYPE go_threads gauge
# TYPE process_cpu_seconds_total counter
# TYPE process_max_fds gauge
# TYPE process_open_fds gauge
# TYPE process_resident_memory_bytes gauge
# TYPE process_start_time_seconds gauge
# TYPE process_virtual_memory_bytes gauge
# TYPE process_virtual_memory_max_bytes gauge
# TYPE pushgateway_build_info gauge
# TYPE pushgateway_http_requests_total counter
` // the newline at the end is required

type typeDefiningReadCloser struct {
	wrapped io.ReadCloser
	header  *bytes.Reader
}

func NewTypeDefiningReadCloser(rc io.ReadCloser) *typeDefiningReadCloser {
	br := bytes.NewReader([]byte(TypeDefinitions))
	return &typeDefiningReadCloser{wrapped: rc, header: br}
}

// Implements https://golang.org/pkg/io/#Reader
func (td *typeDefiningReadCloser) Read(p []byte) (int, error) {
	// If we haven't returned the whole header yet, return
	// as much as will fit in the caller's buffer. Note: the
	// first unit test happens to use a buffer that is smaller
	// than the header, so we know this logic works.
	var n int = 0
	for ; td.header.Len() > 0 && n < len(p); n += 1 {
		p[n], _ = td.header.ReadByte()
	}
	// If we copied any headers, just return them. This avoids
	// correctness issues with the "seam" between the header
	// we inject and the underlying stream content. Note: the
	// bytes.Reader() never returns errors unless the caller
	// reads past the end of the buffer, and we don't.
	if n != 0 {
		return n, nil
	}

	return td.wrapped.Read(p)
}

func (tn *typeDefiningReadCloser) Close() error {
	return tn.wrapped.Close()
}
