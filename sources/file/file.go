package file

import (
	"io"
	"os"

	"github.com/googlegenomics/htsget/block"
)

//FileOffsetReader is a struct that represent a portion of a file specifying the start, length and whether it is virtually closed
type fileOffsetReader struct {
	Start  int64
	Length int64
	File   *os.File
	Closed bool
}

func (f fileOffsetReader) Read(b []byte) (int, error) {
	numBytes := int64(len(b))
	if f.Length <= 0 {
		return 0, io.EOF
	}
	f.Start += numBytes
	f.Length -= numBytes
	return f.File.Read(b)

}

//Close is a no-op function since the file passed to the struct is expected to be closed by external
//TODO not sure if this is a good idea
func (f fileOffsetReader) Close() error {
	//NO-OP file is expected to be closed
	return nil
}

//NewFileRangeReader returns a portion file reader
func NewFileRangeReader(file os.File) block.RangeReader {

	f := fileOffsetReader{
		File:   &file,
		Closed: false,
	}
	return func(start int64, length int64) (io.ReadCloser, error) {
		f.Start = start
		f.Length = length
		_, err := f.File.Seek(start, 0)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
}
