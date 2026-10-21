# Open-source library audit

Every candidate was checked for integration in this environment (Go 1.27,
modernc.org/sqlite pure-Go driver, no cgo by default, Windows + WSL Linux).

| Library | Version | License | Maintenance | Integration result |
|---|---|---|---|---|
| github.com/coder/hnsw | v0.6.1 | MIT | active (Coder) | **integrated, builds, measured** |
| github.com/asg017/sqlite-vec | n/a | Apache-2.0/MIT | active | **not integrated** — requires a loadable C extension; modernc.org/sqlite is pure-Go and does not load extensions in this setup |
| github.com/ncruces/go-sqlite3 (vector ext) | n/a | MIT | active | **not integrated** — would replace the sqlite driver; a driver swap is out of scope for an experiment and risks the whole store |
| hnswlib (C++) + Go bindings | n/a | Apache-2.0 | active | **not integrated** — cgo/native dependency; breaks the static, pure-Go build and cross-platform story |
| fastembed-go | — | Apache-2.0 | — | **not integrated** — needs ONNX Runtime native library + model download |
| onnxruntime_go | — | MIT | active | **not integrated** — cgo + native ONNX Runtime shared library per platform |
| sentence-transformers (BGE-small / MiniLM) | — | Apache-2.0 | active | **not integrated** — Python/ONNX models; redistribution and offline install not validated |
| github.com/viterin/vek | v0.4.2 | MIT | active | pulled in transitively by coder/hnsw; SIMD float32 math |

## Selected

- **coder/hnsw** was selected for *evaluation* (it is the only pure-Go ANN that
  integrated cleanly) and then **rejected on measured recall** (0.17 @10,
  insensitive to `EfSearch`). Kept in the branch as an experimental baseline.

## Rejected / not integrated, and why

- ANN libraries requiring cgo or a loadable extension are incompatible with the
  pure-Go, cross-platform, statically-linked build Beacon ships. Swapping the
  SQLite driver to gain an extension is a large, risky change not justified by
  an experiment.
- Neural embedding runtimes all require either a native shared library, a model
  download, or a hosted service. None satisfies "offline after installation"
  without new native dependencies, and model redistribution/licensing was not
  validated. Left unimplemented rather than faked.

## Compatibility notes

- coder/hnsw v0.6.1 is pure Go (no cgo) and cross-platform; it added four small
  transitive modules to `go.mod`.
- No library introduced a mandatory network call or hosted dependency.

## Recommendation

Before selecting any ANN or neural backend, require: pure-Go or vendored native
binaries for all target platforms, a permissive license permitting
redistribution, offline operation, and a measured recall gate against exact
search. None of the rejected candidates met all four in this pass.
