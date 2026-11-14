// Command parityscan measures zxinggo decode rates and speed over the
// testdata/blackbox corpus, reproducing the PARITY.md methodology:
// per-directory format hints, normal decode only (no PureBarcode-first),
// one run with default options and one with TryHarder.
package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	zxinggo "github.com/fabiomarini/zxinggo"
	"github.com/fabiomarini/zxinggo/binarizer"

	_ "github.com/fabiomarini/zxinggo/aztec"
	_ "github.com/fabiomarini/zxinggo/datamatrix"
	_ "github.com/fabiomarini/zxinggo/maxicode"
	_ "github.com/fabiomarini/zxinggo/oned"
	_ "github.com/fabiomarini/zxinggo/pdf417"
	_ "github.com/fabiomarini/zxinggo/qrcode"
)

type dirSpec struct {
	format zxinggo.Format
	opts   *zxinggo.DecodeOptions
}

func specFor(dir string) (dirSpec, bool) {
	// The add-on requirement (EanAddOnSymbol::Require equivalent) only
	// applies to the extension corpus; everywhere else add-ons are
	// optional (zxing-cpp default Ignore).
	ean := &zxinggo.DecodeOptions{}
	switch {
	case strings.HasPrefix(dir, "aztec"):
		return dirSpec{zxinggo.FormatAztec, nil}, true
	case strings.HasPrefix(dir, "codabar"):
		return dirSpec{zxinggo.FormatCodabar, nil}, true
	case strings.HasPrefix(dir, "code128"):
		return dirSpec{zxinggo.FormatCode128, nil}, true
	case strings.HasPrefix(dir, "code39"):
		return dirSpec{zxinggo.FormatCode39, nil}, true
	case strings.HasPrefix(dir, "code93"):
		return dirSpec{zxinggo.FormatCode93, nil}, true
	case strings.HasPrefix(dir, "datamatrix"):
		return dirSpec{zxinggo.FormatDataMatrix, nil}, true
	case dir == "ean8-1":
		return dirSpec{zxinggo.FormatEAN8, ean}, true
	case dir == "upcean-extension-1":
		return dirSpec{zxinggo.FormatEAN13, &zxinggo.DecodeOptions{AllowedEANExtensions: []int{2, 5}}}, true
	case strings.HasPrefix(dir, "ean13"),
		strings.HasPrefix(dir, "upca"), strings.HasPrefix(dir, "upce"):
		return dirSpec{zxinggo.FormatEAN13, ean}, true
	case strings.HasPrefix(dir, "itf"):
		return dirSpec{zxinggo.FormatITF, nil}, true
	case strings.HasPrefix(dir, "maxicode"):
		return dirSpec{zxinggo.FormatMaxiCode, nil}, true
	case strings.HasPrefix(dir, "pdf417"):
		return dirSpec{zxinggo.FormatPDF417, nil}, true
	case strings.HasPrefix(dir, "qrcode"):
		return dirSpec{zxinggo.FormatQRCode, nil}, true
	case strings.HasPrefix(dir, "rss14"):
		return dirSpec{zxinggo.FormatRSS14, nil}, true
	case strings.HasPrefix(dir, "rssexpanded"):
		return dirSpec{zxinggo.FormatRSSExpanded, nil}, true
	case dir == "inverted":
		return dirSpec{zxinggo.FormatDataMatrix, &zxinggo.DecodeOptions{AlsoInverted: true}}, true
	}
	return dirSpec{}, false
}

// formatFor returns the format hint list for a directory. EAN/UPC families
// hint their concrete formats so the per-directory counts stay meaningful.
func formatsFor(dir string, s dirSpec) []zxinggo.Format {
	switch {
	case strings.HasPrefix(dir, "ean13"), strings.HasPrefix(dir, "upcean-extension"):
		return []zxinggo.Format{zxinggo.FormatEAN13}
	case strings.HasPrefix(dir, "ean8"):
		return []zxinggo.Format{zxinggo.FormatEAN8}
	case strings.HasPrefix(dir, "upca"):
		return []zxinggo.Format{zxinggo.FormatUPCA}
	case strings.HasPrefix(dir, "upce"):
		return []zxinggo.Format{zxinggo.FormatUPCE}
	}
	return []zxinggo.Format{s.format}
}

func loadExpected(basePath string) (string, bool) {
	if data, err := os.ReadFile(basePath + ".txt"); err == nil {
		return string(data), true
	}
	data, err := os.ReadFile(basePath + ".bin")
	if err != nil {
		return "", false
	}
	runes := make([]rune, len(data))
	for i, b := range data {
		runes[i] = rune(b)
	}
	return string(runes), true
}

type imageFile struct {
	path     string
	expected string
	segment  bool // multi-symbol segment: text must be contained in group text
}

func collectImages(dir string) []imageFile {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []imageFile
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif":
		default:
			continue
		}
		base := filepath.Join(dir, e.Name()[:len(e.Name())-len(ext)])
		exp, ok := loadExpected(base)
		seg := false
		if !ok {
			// Multi-symbol segment image (e.g. pdf417-4/01-01.png): expected
			// text lives at the group base (01.bin); a segment decode is OK
			// when its text is contained in the group text.
			if i := strings.LastIndex(base, "-"); i > 0 {
				if exp, ok = loadExpected(base[:i]); ok {
					seg = true
				}
			}
			if !ok {
				continue
			}
		}
		out = append(out, imageFile{filepath.Join(dir, e.Name()), exp, seg})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func decodeOne(path string, formats []zxinggo.Format, base *zxinggo.DecodeOptions, tryHarder bool) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return "", false
	}
	// PARITY methodology: PureBarcode hint first, then normal decode.
	bitmap := zxinggo.NewBinaryBitmap(binarizer.NewHybrid(zxinggo.NewImageLuminanceSource(img)))
	for _, pure := range []bool{true, false} {
		opts := &zxinggo.DecodeOptions{PossibleFormats: formats, TryHarder: tryHarder, PureBarcode: pure}
		if base != nil {
			opts.AllowedEANExtensions = base.AllowedEANExtensions
			opts.AlsoInverted = base.AlsoInverted
		}
		result, err := zxinggo.Decode(bitmap, opts)
		if err == nil && result != nil {
			return result.Text, true
		}
	}
	return "", false
}

func run(root, onlyDir string, tryHarder bool) {
	dumpRun(root, onlyDir, tryHarder, "")
}

func dumpRun(root, onlyDir string, tryHarder bool, dumpPath string) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var dumpF *os.File
	if dumpPath != "" {
		var ferr error
		dumpF, ferr = os.Create(dumpPath)
		if ferr != nil {
			fmt.Fprintln(os.Stderr, ferr)
			os.Exit(1)
		}
		defer dumpF.Close()
	}
	var names []string
	for _, d := range dirs {
		if d.IsDir() && (onlyDir == "" || d.Name() == onlyDir) {
			names = append(names, d.Name())
		}
	}
	sort.Strings(names)

	total, okTotal := 0, 0
	start := time.Now()
	for _, name := range names {
		spec, known := specFor(name)
		if !known {
			fmt.Printf("%-24s SKIP (no format mapping)\n", name)
			continue
		}
		images := collectImages(filepath.Join(root, name))
		if len(images) == 0 {
			continue
		}
		ok := 0
		type segment struct {
			idx     int
			text    string
			decoded bool
		}
		groups := map[string][]segment{}
		for _, img := range images {
			text, decoded := decodeOne(img.path, formatsFor(name, spec), spec.opts, tryHarder)
			if dumpF != nil && !img.segment {
				if decoded {
					fmt.Fprintf(dumpF, "%s/%s\t%s\n", name, filepath.Base(img.path), strings.ReplaceAll(text, "\n", `\n`))
				} else {
					fmt.Fprintf(dumpF, "%s/%s\n", name, filepath.Base(img.path))
				}
			}
			if !img.segment {
				if decoded && text == img.expected {
					ok++
				}
				continue
			}
			base := filepath.Base(img.path)
			idx := 0
			if dash := strings.LastIndex(base, "-"); dash > 0 {
				fmt.Sscanf(base[dash+1:], "%d", &idx)
			}
			groups[img.expected] = append(groups[img.expected], segment{idx, text, decoded})
		}
		for exp, segs := range groups {
			sort.Slice(segs, func(i, j int) bool { return segs[i].idx < segs[j].idx })
			var sb strings.Builder
			complete := true
			for _, s := range segs {
				if !s.decoded {
					complete = false
				}
				sb.WriteString(s.text)
			}
			if complete && sb.String() == exp {
				ok += len(segs)
			}
		}
		total += len(images)
		okTotal += ok
		fmt.Printf("%-24s %3d/%-3d\n", name, ok, len(images))
	}
	fmt.Printf("%-24s %3d/%-3d  (%.2f ms/image)\n\n", "TOTAL", okTotal, total,
		float64(time.Since(start).Milliseconds())/float64(max(total, 1)))
}

func main() {
	root := flag.String("dir", "testdata/blackbox", "blackbox corpus root (or a single directory with -format)")
	only := flag.String("format", "", "run only this subdirectory of the root")
	TH := flag.Bool("try-harder", false, "run only with TryHarder")
	cpuProfile := flag.String("cpuprofile", "", "write CPU profile to file")
	dump := flag.String("dump", "", "write per-image decoded texts (TryHarder run) to file")
	flag.Parse()
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if !*TH {
		fmt.Println("=== default options ===")
		run(*root, *only, false)
	}
	fmt.Println("=== TryHarder ===")
	dumpRun(*root, *only, true, *dump)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
