package transfer

import (
	"io"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	pion "github.com/pion/webrtc/v4"
	"github.com/vmihailenco/msgpack/v5"
)

type ChunkSender struct {
	channel    *pion.DataChannel
	controller *utils.ChunkSizeController
	buffer     []byte
}

func NewChunkSender(dc *pion.DataChannel) *ChunkSender {
	dc.SetBufferedAmountLowThreshold(uint64(LowWaterMark))
	return &ChunkSender{
		channel:    dc,
		controller: utils.NewChunkSizeController(),
		buffer:     make([]byte, utils.MaxChunkSize),
	}
}

func (s *ChunkSender) WaitForWindow() error {
	bufferedAmount := s.channel.BufferedAmount()
	if bufferedAmount < uint64(HighWaterMark) {
		return nil
	}

	wait := make(chan struct{}, 1)
	s.channel.OnBufferedAmountLow(func() {
		select {
		case wait <- struct{}{}:
		default:
		}
	})

	timeout := time.Duration(SendTimeout) * time.Second
	select {
	case <-wait:
		return nil
	case <-time.After(timeout):
		newBufferedAmount := s.channel.BufferedAmount()
		if newBufferedAmount < bufferedAmount {
			return nil
		}
		return WrapError("send", ErrBufferTimeout, "buffer not draining")
	}
}

func (s *ChunkSender) WaitForDrain() {
	start := time.Now()
	for s.channel.BufferedAmount() > 0 && time.Since(start) < time.Duration(DrainTimeout)*time.Second {
		if s.channel.ReadyState() != pion.DataChannelStateOpen {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *ChunkSender) IsOpen() bool {
	return s.channel.ReadyState() == pion.DataChannelStateOpen
}

func (s *ChunkSender) GetChunkSize() int {
	return s.controller.GetChunkSize()
}

func (s *ChunkSender) RecordBytes(n int64) {
	s.controller.RecordBytesTransferred(n)
}

func (s *ChunkSender) Buffer() []byte {
	return s.buffer
}

func (s *ChunkSender) Send(data []byte) error {
	return s.channel.Send(data)
}

type SingleChannelFileSender struct {
	sender   *ChunkSender
	fileName string
	fileSize int64
}

func NewSingleChannelFileSender(dc *pion.DataChannel, fileName string, fileSize int64) *SingleChannelFileSender {
	return &SingleChannelFileSender{
		sender:   NewChunkSender(dc),
		fileName: fileName,
		fileSize: fileSize,
	}
}

func (s *SingleChannelFileSender) SendChunks(file io.Reader, offset uint64, onProgress func(uint64), onComplete func(), onError func(string)) error {
	if !s.sender.IsOpen() {
		onError("channel not open")
		return ErrChannelNotOpen
	}

	currentOffset := offset
	for {
		if !s.sender.IsOpen() {
			onError("channel closed")
			return ErrChannelClosed
		}

		if err := s.sender.WaitForWindow(); err != nil {
			onError("buffer timeout")
			return err
		}

		chunkSize := s.sender.GetChunkSize()
		n, err := file.Read(s.sender.Buffer()[:chunkSize])

		if err != nil {
			if err == io.EOF {
				s.sender.WaitForDrain()
				onComplete()
				return nil
			}
			onError(err.Error())
			return err
		}

		final := currentOffset+uint64(n) >= uint64(s.fileSize)
		message, err := webrtc.NewMessage(MessageTypeChunk, webrtc.ChunkPayload{
			FileName: s.fileName,
			Offset:   currentOffset,
			Bytes:    s.sender.Buffer()[:n],
			Final:    final,
		})
		if err != nil {
			onError(err.Error())
			return err
		}

		data, err := msgpack.Marshal(message)
		if err != nil {
			onError(err.Error())
			return err
		}

		if err := s.sender.Send(data); err != nil {
			onError(err.Error())
			return err
		}

		currentOffset += uint64(n)
		s.sender.RecordBytes(int64(n))
		onProgress(currentOffset)
	}
}

type MultiChannelFileSender struct {
	sender *ChunkSender
}

func NewMultiChannelFileSender(dc *pion.DataChannel) *MultiChannelFileSender {
	return &MultiChannelFileSender{
		sender: NewChunkSender(dc),
	}
}

func (s *MultiChannelFileSender) SendChunks(file io.Reader, onProgress func(int64), onComplete func(), onError func(string)) error {
	if !s.sender.IsOpen() {
		onError("channel not open")
		return ErrChannelNotOpen
	}

	var sentBytes int64
	for {
		if !s.sender.IsOpen() {
			onError("channel closed")
			return ErrChannelClosed
		}

		if err := s.sender.WaitForWindow(); err != nil {
			onError("buffer timeout")
			return err
		}

		chunkSize := s.sender.GetChunkSize()
		n, err := file.Read(s.sender.Buffer()[:chunkSize])

		if err != nil {
			if err == io.EOF {
				s.sender.WaitForDrain()
				onComplete()
				return nil
			}
			onError(err.Error())
			return err
		}

		if err := s.sender.Send(s.sender.Buffer()[:n]); err != nil {
			onError(err.Error())
			return err
		}

		sentBytes += int64(n)
		s.sender.RecordBytes(int64(n))
		onProgress(sentBytes)
	}
}
