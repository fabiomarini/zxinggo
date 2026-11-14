package maxicode

import zxinggo "github.com/fabiomarini/zxinggo"

func init() {
	zxinggo.RegisterReader(zxinggo.FormatMaxiCode, func(opts *zxinggo.DecodeOptions) zxinggo.Reader {
		return NewReader()
	})
}
