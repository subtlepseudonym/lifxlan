package lifxlan

import (
	"context"
	"fmt"
	"net"
)

// WaitForAcks helps device API implementations to wait for acks.
//
// It blocks until acks for all sequences are received,
// in which case it returns nil error.
// It also returns when the context is cancelled.
//
// This function drops all received messages that is not an ack,
// or ack messages that the sequence and source don't match.
// Therefore, there shouldn't be more than one WaitForAcks functions running for
// the same connection at the same time,
// and this function should only be used when no other responses are expected.
//
// If this function returns an error,
// the error would be of type *WaitForAcksError.
func WaitForAcks(
	ctx context.Context,
	conn net.Conn,
	source uint32,
	sequences ...uint8,
) error {
	e := &WaitForAcksError{
		Total: len(sequences),
	}

	select {
	default:
	case <-ctx.Done():
		e.Cause = ctx.Err()
		return e
	}

	if len(sequences) == 0 {
		return nil
	}

	seqMap := make(map[uint8]bool)
	for _, seq := range sequences {
		seqMap[seq] = true
	}

	buf := make([]byte, ResponseReadBufferSize)
	for {
		select {
		default:
		case <-ctx.Done():
			e.Cause = ctx.Err()
			return e
		}

		if err := conn.SetReadDeadline(GetReadDeadline()); err != nil {
			e.Cause = err
			return e
		}

		n, err := conn.Read(buf)
		if err != nil {
			if CheckTimeoutError(err) {
				continue
			}
			e.Cause = err
			return e
		}

		resp, err := ParseResponse(buf[:n])
		if err != nil {
			e.Cause = err
			return e
		}
		if resp.Source != source || resp.Message != Acknowledgement {
			continue
		}
		if seqMap[resp.Sequence] {
			e.Received++
			delete(seqMap, resp.Sequence)
			if len(seqMap) == 0 {
				// All ack received.
				return nil
			}
		}
	}
}

// WaitForAcksError defines the error returned by WaitForAcks.
type WaitForAcksError struct {
	Received int
	Total    int
	Cause    error
}

var _ error = (*WaitForAcksError)(nil)

func (e *WaitForAcksError) Error() string {
	return fmt.Sprintf(
		"lifxlan.WaitForAcks: %d of %d ack(s) received: %v",
		e.Received,
		e.Total,
		e.Cause,
	)
}
