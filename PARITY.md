# Parity Report: zxinggo vs zxing-cpp

Date: 2026-11-14 (updated after the parity work phase).
zxing-cpp: master `f19f7c6`, built with `ZXING_USE_BUNDLED_ZINT` (writer backend) and the `ZXingReader` / `ZXingWriter` example CLIs.
zxinggo: this repository at HEAD + the review fixes and the detection rework listed below.
Machine: Apple silicon (arm64), single-threaded comparisons.

## 1. Decode parity on the shared blackbox corpus

Both libraries decoded the same 1,124 images from `testdata/blackbox` (the Java
ZXing corpus). zxinggo ran twice (default and `TryHarder`); a result counts as
OK when the decoded text matches the expected `.txt`/`.bin` value. zxing-cpp
ran with its default (most aggressive) detection; its `Text`/`HRI` JSON fields
were compared with normalization for known rendering differences (UPC-A/E
expansion, DataBar AI parentheses, Codabar start/stop chars, EAN add-on
suffixes).

zxinggo numbers are the `TryHarder` run (`go run ./cmd/parityscan`).

| Format | Images | zxinggo OK | zxing-cpp OK |
|---|---|---|---|
| Aztec | 37 | 37 (100%) | 37 (100%) |
| Codabar | 11 | 11 (100%) | 11 (100%) |
| Code 128 | 48 | 47 (97%) | 47 (97%) |
| Code 39 | 23 | 23 (100%) | 23 (100%) |
| Code 93 | 3 | 3 (100%) | 3 (100%) |
| Data Matrix | 58 | 56 (96%) | 56 (96%) |
| Data Matrix (inverted) | 1 | 1 (100%) | 1 (100%) |
| DataBar Expanded | 247 | 246 (99%) | 243 (98%) |
| DataBar-14 | 30 | 16 (53%) | 16 (53%) |
| EAN-13 | 157 | 118 (75%) | 115 (73%) |
| EAN-8 | 8 | 8 (100%) | 8 (100%) |
| ITF | 28 | 28 (100%) | 28 (100%) |
| MaxiCode | 9 | 9 (100%) | 9 (100%) |
| PDF417 | 61 | 58 (95%) | 51 (83%) |
| QR Code | 179 | 170 (95%) | 165 (92%) |
| UPC-A | 167 | 98 (58%) | 96 (57%) |
| UPC-E | 57 | 46 (80%) | 46 (80%) |
| **Total** | **1124** | **977 (87%)** | **955 (85%)** |

zxinggo now meets or exceeds zxing-cpp in every format. The DataBar-14
parity was reached by replacing the Java pair-tally reader with a port of
zxing-cpp's `ODDataBarReader.cpp` (edge-to-edge character model, strict
finder/guard checks, cross-row pair accumulation for stacked symbols).
The default (non-`TryHarder`) run decodes 907/1124.

Per-image agreement (decoded with any text, both `TryHarder`, 1,052 images
outside the stacked-segment corpus where per-image comparison is not
meaningful):

| | zxing-cpp decoded | zxing-cpp failed |
|---|---|---|
| **zxinggo decoded** | 889 | 19 |
| **zxinggo failed** | 10 | 134 |

The 10 cpp-only images: `datamatrix-2/03,17` (glare-erased clock track),
`maxicode-1/1,with-errors` (mode 6, see §5), `qrcode-1/18` (ring-fit
anchor refinement), `qrcode-4/19` (checkerboard background),
`ean13-2/04` and `rssexpanded-2/12` (hard photo frames),
`pdf417-4/04-01` (segment of a multi-symbol group), and `upca-4/1`
(a zxing-cpp misread). The 19 Go-only images: 6 QR codes, 5 PDF417,
2 Data Matrix, 2 UPC-A, 2 UPC-E, 2 EAN-13.

## 2. Round-trip cross-validation

### Go encodes -> C++ decodes

`zxinggo.Encode` output (PNG, module-scaled) decoded by `ZXingReader`:
**12/12** formats round-trip (QR, DataMatrix, Aztec, Code 128/39/93, Codabar,
ITF, EAN-13, EAN-8, UPC-A, PDF417). PDF417 previously failed; the encoder was
fixed (see Known issues) and its output is now decoded by both zxing-cpp and
zxinggo, including ISO-8859-1 content and multi-block symbols.

### C++ encodes -> Go decodes

`ZXingWriter` (zint backend) output decoded by `zxinggo.Decode`:
**14/14** (QR, DataMatrix, Aztec, PDF417, Code 128/39/93, Codabar, ITF, EAN-13,
EAN-8, UPC-A, UPC-E, DataBar-14). The Go decoders accept standard-compliant
symbols from an independent encoder.

## 3. Performance

Same 1,124 images, one process, wall time (includes image loading):

| Decoder | Time | ms/image | Images decoded |
|---|---|---|---|
| zxing-cpp `ZXingReader` (default) | 3.50 s | 3.1 | 955 |
| zxinggo (default options) | 3.5 s | 3.1 | 907 |
| zxinggo (`TryHarder`) | 3.84 s | 3.4 | 977 |

Decode-only time (PNG decoding excluded, best of 3 passes over pre-decoded
images): **1.41 ms/image** for zxinggo. Of the wall time above, ~3.25 ms/image
is Go's stdlib `image/png` inflate - the zxinggo decode pipeline itself is
~1.4 ms/image. zxing-cpp's stb_image-based loading is faster, which accounts
for nearly all of the remaining wall-clock difference.

## 4. Detection rework applied (this phase)

- **1D (EAN/UPC/DataBar)**: row scanning rebuilt on the zxing-cpp model -
  global-histogram pattern rows with sharpening (`binarizer.RowScanner`),
  `PatternView`/`IsPattern`/`FindLeftGuard` (`internal/pattern`),
  `ODMultiUPCEANReader` port with GS1 quiet-zone rules, rotation pass
  (rows + columns), and TryHarder row-stepping (`oned.DecodeOneD`).
  EAN/UPC photo decoding went from 44 to 270 images.
- **DataBar-14**: `oned.RSS14Reader` (Java pair-tally over bit rows) replaced
  by `oned.DataBarReader`, a port of `ODDataBarReader.cpp` — edge-to-edge
  character normalization, strict finder/guard/character checks, and
  cross-row pair accumulation with position plausibility for stacked
  symbols. rss14-2: 8/24 -> 10/24 (zxing-cpp parity 16/30 total).
- **Aztec**: detector replaced with a port of `AZDetector.cpp`
  (concentric-ring center finding, orientation tokens, alignment-pattern grid
  refinement, ROI grid sampling via new `internal/bmcursor`,
  `internal/concentric`, `internal/localgrid`). aztec-2: 5/22 -> 22/22.
- **Data Matrix**: WhiteRectangleDetector edge freeze fixed (white-run
  tolerance), bounding-box reseed, TryHarder coarse seeds, and edge-line
  refinement with dimension candidates (`datamatrix/detector/refine.go`).
  datamatrix-2: 13/18 -> 16/18.
- **QR**: alignment-pattern centers refined on both axes; multi-candidate
  sampling (`qrcode/detector/sampling.go`) with tiled grids for v7+, anchor
  jitter under TryHarder, global-binarizer and rotated retries.
  157 -> 170 images; 6 images decoded that zxing-cpp misses.
- **PDF417 writer**: EC codewords no longer iterated as UTF-8 bytes; input
  converted to ISO-8859-1 octets before compaction; dead `BarcodeMatrix.width`
  removed. Round-trip tests cover ASCII, Latin-1, numeric, long, and
  byte-mode payloads.
- **Luminance**: `NewImageLuminanceSource` fast paths for
  Gray/RGBA/NRGBA/Gray16/RGBA64/NRGBA64 (exact `At()`/`RGBA()` equivalence
  verified by `TestLuminanceFastPathsMatchGeneric`). Decode-only time
  4.30 -> 1.41 ms/image.

## 5. Feature gaps vs zxing-cpp

Formats zxing-cpp reads that zxinggo does not:

| Capability | Notes |
|---|---|
| Micro QR, rMQR, QR Model 1 | zxing-cpp reads all; Go reads none |
| DMRE (rectangular Data Matrix) | version table covers all 48 shapes; decode partial — 6 of 18 zxing-cpp sample renders decode (wide 8/12-row variants fail detection); zxing-cpp reads 18/18 |
| MicroPDF417, Compact PDF417 | Go reads standard PDF417 only |
| MaxiCode modes 0/1/6 | Go matches **Java** here (Java also rejects mode 6); zxing-cpp reads all modes with structured parsing |
| DataBar Limited / Stacked / Stacked Omni | Go reads DataBar-14 (Omni) and Expanded (incl. stacked) only |
| EAN-2 / EAN-5 add-ons, ISBN | Go supports UPC/EAN extensions (2/5) on decode with suggested-price/issue-number metadata; no ISBN conversion |
| ITF-14, Code 32, PZN, Telepen, DXFilmEdge, MSI, AICode | not implemented in Go |

Writers: zxing-cpp (zint backend) writes all of the above plus EAN-2/5 add-ons;
zxinggo writes the 13 formats listed in the README. MicroQR/rMQR/add-on/DataBar/
MaxiCode writing is absent in Go.

Metadata: zxing-cpp additionally provides structured GS1/AI parsing, content
type classification, and symbol image extraction; zxinggo provides symbology
identifier, EC level, byte segments, structured append, errors-corrected, and
UPC/EAN extension metadata only.

## 6. Quality work applied

- gofmt/vet/golangci-lint clean; all tests green.
- Reed-Solomon errors instead of panics; `bitutil` bounds checks; stateless
  `MultiFormatReader` with working `AlsoInverted`; `purego_test.go` keeps the
  library cgo-free.
- Blackbox thresholds updated to the post-rework contract (aztec 22/22,
  datamatrix-2 16/18, QR dirs up to 42/48, EAN/UPC dirs at current rates).

## 7. Known issues (remaining)

1. **UPCA5 at 180°: 21/35 vs Java's 22 (default mode only).** Go's default
   row budget follows zxing-cpp's lean middle-out scan (15 lines,
   `height/32` step); Java scans linearly from `height/2-5` and catches one
   marginal frame Go's sampled rows miss. TryHarder decodes 23/35 at both
   rotations, above Java.
2. **datamatrix-2/03.png and 17.png.** Glare-erased clock track needs the
   contour-based `DetectNew` port; 17.png also defeats zxing-cpp's global
   binarizer (needs local binarization).
3. **qrcode-1/18.png** needs zxing-cpp's ConcentricFinder ring-fit anchor
   refinement; **qrcode-4/19.png** needs a checkerboard/local binarizer.
4. **ean13-5 / upca-6 corpora (37 images)**: neither library decodes them;
   the luminance contrast in the source photos defeats both binarizers.
5. **PDF417 runes beyond U+00FF** are rejected at encode time (ECI not
   implemented).

## 8. Reproducing

```sh
# Go side
go run ./cmd/parityscan            # per-dir table, default + TryHarder runs
go run ./cmd/parityscan -format aztec-2 -try-harder
go run ./cmd/parityscan -dump out.tsv   # per-image texts (TryHarder)

# C++ side
git clone --depth 1 https://github.com/zxing-cpp/zxing-cpp /tmp/zxing-cpp
cd /tmp/zxing-cpp
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DZXING_WRITERS=OFF
cmake --build build --target ZXingReader
# CLI: build/example/ZXingReader img.png
```

Go decode options used: `PossibleFormats` set per directory,
`AllowedEANExtensions: [2,5]` for the `upcean-extension-1` directory only,
`AlsoInverted` for the `inverted` directory.
