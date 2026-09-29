package runtime

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
)

// compressFleetRecovery bounds private recovery storage without changing its decoded contract.
func compressFleetRecovery(data []byte) ([]byte, error) {
	return compressRecoveryRecord(data, MaxFleetRecoveryBytes)
}

func compressRecoveryRecord(data []byte, maximum int) ([]byte, error) {
	if len(data) == 0 || len(data) > maximum {
		return nil, invalidInputPublication("fleet recovery exceeds the decoded input byte bound")
	}
	var output bytes.Buffer
	writer, err := gzip.NewWriterLevel(&output, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if output.Len() > maximum {
		return nil, invalidInputPublication("fleet recovery exceeds the compressed input byte bound")
	}
	return output.Bytes(), nil
}

func decompressFleetRecovery(ctx context.Context, data []byte, limit int64) ([]byte, error) {
	return decompressRecoveryRecord(ctx, data, MaxFleetRecoveryBytes, limit)
}

func decompressRecoveryRecord(ctx context.Context, data []byte, maximum int, limit int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maximum || limit <= 0 || limit > int64(maximum) {
		return nil, invalidInputPublication("fleet recovery exceeds the compressed input byte bound")
	}
	input := bytes.NewReader(data)
	reader, err := gzip.NewReader(input)
	if err != nil {
		return nil, invalidInputPublication("fleet recovery has an invalid compression header")
	}
	reader.Multistream(false)
	decoded, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err := errors.Join(err, reader.Close()); err != nil {
		return nil, err
	}
	if len(decoded) == 0 || int64(len(decoded)) > limit || input.Len() != 0 {
		return nil, invalidInputPublication("fleet recovery has excess decoded bytes or trailing data")
	}
	return decoded, ctx.Err()
}
