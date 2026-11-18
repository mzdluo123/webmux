package voice

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"
)

// ErrAudioBufferFull reports that the subprocess STDIN queue is full because
// the speech recognizer is not draining audio fast enough.
var ErrAudioBufferFull = errors.New("voice audio buffer full")

const (
	// sessionAudioQueueCapacity bounds pending PCM chunks between the WebSocket
	// reader and the subprocess STDIN writer (~5 seconds of 20ms frames).
	sessionAudioQueueCapacity = 256
	// maxQueuedAudioBytes caps total unread PCM bytes buffered in memory (~16
	// seconds of 16kHz mono s16le audio) before backpressure applies.
	maxQueuedAudioBytes = 512 * 1024
	// audioEnqueueTimeout bounds how long WriteAudio waits when the in-memory
	// PCM queue is full so a stalled recognizer never starves control frames.
	audioEnqueueTimeout = 250 * time.Millisecond
)

// Session is one active recording subprocess.
type Session struct {
	cancel     context.CancelFunc
	mu         sync.Mutex
	stdin      io.WriteCloser
	queue      [][]byte
	queuedSize int
	spaceWait  chan struct{}
	workReady  chan struct{}
	inputStop  bool
	aborted    bool
	writeErr   error
	closed     bool
	stopTimer  *time.Timer
	writerDone chan struct{}
	events     chan Event
	done       chan struct{}
}

func newSession(cancel context.CancelFunc, stdin io.WriteCloser) *Session {
	sess := &Session{
		cancel:     cancel,
		stdin:      stdin,
		spaceWait:  make(chan struct{}),
		workReady:  make(chan struct{}, 1),
		writerDone: make(chan struct{}),
		events:     make(chan Event, sessionEventQueueCapacity),
		done:       make(chan struct{}),
	}
	go sess.writeLoop(stdin)
	return sess
}

func (s *Session) detachStdinLocked() io.WriteCloser {
	stdin := s.stdin
	s.stdin = nil
	return stdin
}

func (s *Session) wakeWriterLocked() {
	select {
	case s.workReady <- struct{}{}:
	default:
	}
}

func (s *Session) wakeSpaceLocked() {
	close(s.spaceWait)
	s.spaceWait = make(chan struct{})
}

// Events returns the channel of recognition events. It is closed when the
// subprocess exits and all output has been processed.
func (s *Session) Events() <-chan Event {
	return s.events
}

// WriteAudio queues a chunk of 16kHz mono s16le PCM for the subprocess STDIN.
func (s *Session) WriteAudio(pcm []byte) error {
	if len(pcm) == 0 {
		return nil
	}
	chunk := bytes.Clone(pcm)
	var timer *time.Timer
	var timeoutC <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		s.mu.Lock()
		if s.writeErr != nil {
			err := s.writeErr
			s.mu.Unlock()
			return err
		}
		if s.inputStop || s.aborted || s.stdin == nil {
			s.mu.Unlock()
			return io.ErrClosedPipe
		}
		if len(s.queue) < sessionAudioQueueCapacity && (len(s.queue) == 0 || s.queuedSize+len(chunk) <= maxQueuedAudioBytes) {
			s.queue = append(s.queue, chunk)
			s.queuedSize += len(chunk)
			s.wakeWriterLocked()
			s.mu.Unlock()
			return nil
		}
		waitCh := s.spaceWait
		s.mu.Unlock()

		if timer == nil {
			timer = time.NewTimer(audioEnqueueTimeout)
			timeoutC = timer.C
		}
		select {
		case <-waitCh:
		case <-timeoutC:
			return ErrAudioBufferFull
		}
	}
}

// StopInput ends audio input so the writer flushes queued PCM chunks and closes
// STDIN, and starts the finish timeout timer immediately so a hung subprocess
// cannot block completion indefinitely.
func (s *Session) StopInput(timeout time.Duration) {
	s.mu.Lock()
	if !s.closed {
		s.inputStop = true
		if s.stopTimer == nil && timeout > 0 {
			s.stopTimer = time.AfterFunc(timeout, s.abortFromTimer)
		}
		s.wakeWriterLocked()
		s.wakeSpaceLocked()
	}
	s.mu.Unlock()
}

func (s *Session) abortFromTimer() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.aborted = true
	s.queue = nil
	s.queuedSize = 0
	toClose := s.detachStdinLocked()
	s.wakeWriterLocked()
	s.wakeSpaceLocked()
	s.mu.Unlock()
	if toClose != nil {
		_ = toClose.Close()
	}
	s.cancel()
}

func (s *Session) abortInput() {
	s.mu.Lock()
	s.aborted = true
	s.queue = nil
	s.queuedSize = 0
	toClose := s.detachStdinLocked()
	s.wakeWriterLocked()
	s.wakeSpaceLocked()
	s.mu.Unlock()
	if toClose != nil {
		_ = toClose.Close()
	}
}

func (s *Session) stopFinishTimer() {
	s.mu.Lock()
	timer := s.stopTimer
	s.stopTimer = nil
	s.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
}

// Close terminates the subprocess and waits for its reader and writer
// goroutines to exit.
func (s *Session) Close() {
	s.mu.Lock()
	alreadyClosed := s.closed
	s.closed = true
	s.aborted = true
	s.queue = nil
	s.queuedSize = 0
	toClose := s.detachStdinLocked()
	timer := s.stopTimer
	s.stopTimer = nil
	s.wakeWriterLocked()
	s.wakeSpaceLocked()
	s.mu.Unlock()

	if timer != nil {
		timer.Stop()
	}
	if toClose != nil {
		_ = toClose.Close()
	}
	if !alreadyClosed {
		s.cancel()
	}
	<-s.done
}

func (s *Session) writeLoop(stdin io.Writer) {
	defer close(s.writerDone)
	for {
		s.mu.Lock()
		for len(s.queue) == 0 && !s.inputStop && !s.aborted {
			s.mu.Unlock()
			<-s.workReady
			s.mu.Lock()
		}
		if s.aborted {
			s.queue = nil
			s.queuedSize = 0
			toClose := s.detachStdinLocked()
			s.wakeSpaceLocked()
			s.mu.Unlock()
			if toClose != nil {
				_ = toClose.Close()
			}
			return
		}
		if len(s.queue) == 0 {
			toClose := s.detachStdinLocked()
			s.mu.Unlock()
			if toClose != nil {
				_ = toClose.Close()
			}
			return
		}
		chunk := s.queue[0]
		s.queue[0] = nil
		s.queue = s.queue[1:]
		if len(s.queue) == 0 {
			s.queue = nil
		}
		s.queuedSize -= len(chunk)
		s.wakeSpaceLocked()
		s.mu.Unlock()

		if _, err := stdin.Write(chunk); err != nil {
			s.mu.Lock()
			if !s.aborted && !errors.Is(err, os.ErrClosed) {
				s.writeErr = err
			}
			s.aborted = true
			s.queue = nil
			s.queuedSize = 0
			toClose := s.detachStdinLocked()
			s.wakeSpaceLocked()
			s.mu.Unlock()
			if toClose != nil {
				_ = toClose.Close()
			}
			return
		}
	}
}
