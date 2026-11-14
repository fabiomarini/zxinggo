package qrcode

import zxinggo "github.com/fabiomarini/zxinggo"

func init() {
	zxinggo.RegisterReader(zxinggo.FormatQRCode, func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewReader()
	})
	zxinggo.RegisterWriter(zxinggo.FormatQRCode, func() zxinggo.Writer {
		return NewWriter()
	})
}
