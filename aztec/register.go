package aztec

import zxinggo "github.com/fabiomarini/zxinggo"

func init() {
	zxinggo.RegisterReader(zxinggo.FormatAztec, func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewReader()
	})
	zxinggo.RegisterWriter(zxinggo.FormatAztec, func() zxinggo.Writer {
		return NewWriter()
	})
}
