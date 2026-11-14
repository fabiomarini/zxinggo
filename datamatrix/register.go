package datamatrix

import zxinggo "github.com/fabiomarini/zxinggo"

func init() {
	zxinggo.RegisterReader(zxinggo.FormatDataMatrix, func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewReader()
	})
	zxinggo.RegisterWriter(zxinggo.FormatDataMatrix, func() zxinggo.Writer {
		return NewWriter()
	})
}
