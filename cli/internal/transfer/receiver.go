package transfer

import (
	"os"
	"path/filepath"

	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
)

type FileWriter struct {
	File          *os.File
	Metadata      webrtc.FileMetadata
	ReceivedBytes uint64
	Index         int
}

func NewFileWriter(meta webrtc.FileMetadata, index int, opts *TransferOptions) (*FileWriter, error) {
	filename := utils.GetUniqueFilename(meta.Name)
	if opts != nil && opts.OutputDir != "" {
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			return nil, NewFileError("create directory", opts.OutputDir, err)
		}
		filename = filepath.Join(opts.OutputDir, filename)
	}

	file, err := os.Create(filename)
	if err != nil {
		return nil, NewFileError("create file", meta.Name, err)
	}

	return &FileWriter{
		File:     file,
		Metadata: meta,
		Index:    index,
	}, nil
}

func (w *FileWriter) Write(data []byte) (int, error) {
	n, err := w.File.Write(data)
	if err != nil {
		return n, NewFileError("write", w.Metadata.Name, err)
	}
	w.ReceivedBytes += uint64(n)
	return n, nil
}

func (w *FileWriter) WriteAt(data []byte, offset uint64) (int, error) {
	if offset != w.ReceivedBytes {
		if _, err := w.File.Seek(int64(offset), 0); err != nil {
			return 0, NewFileError("seek", w.Metadata.Name, err)
		}
		w.ReceivedBytes = offset
	}
	return w.Write(data)
}

func (w *FileWriter) IsComplete() bool {
	return w.ReceivedBytes >= w.Metadata.Size
}

func (w *FileWriter) Close() error {
	return w.File.Close()
}
