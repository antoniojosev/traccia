// Package traccia holds assets embedded into the compiled binary. The
// tracking script must live under the module root (not internal/) because
// //go:embed patterns can't contain ".." — this is the only location from
// which sdk/js/src/traccia.js is reachable without one.
package traccia

import _ "embed"

//go:embed sdk/js/src/traccia.js
var TrackingScript []byte
