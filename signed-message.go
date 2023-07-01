package main

import (
	"bytes"
	"compress/zlib"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

var secretSignature = []byte("E7zbgnIhe4SFklYLHjM35dwEuRCIxPzelhkGcnZ6I737Osgbk04fG7woSqIJmsJs6dACYIkHYFBvPXIZ")

func DecodeSignedMessage(b64 string, into any) error {
	if len(b64) < 4 {
		return signedMessageTooSmallError(len(b64))
	}
	if len(b64) > 64 {
		return signedMessageTooLargeError(len(b64))
	}
	data, err := base64.URLEncoding.DecodeString(b64)
	if err != nil {
		return err
	}
	compressed, sig := data[:len(data)-4], data[len(data)-4:]
	ok, err := Verify(compressed, sig)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("невалидная сигнатура: %v | %v", string(compressed), string(sig))
	}
	gz, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return err
	}
	defer gz.Close()
	jd := basicMessageDecoder(gz)
	if err := jd.Decode(into); err != nil {
		return err
	}
	return nil
}

type signedMessageTooLargeError int

func (e signedMessageTooLargeError) Error() string {
	return fmt.Sprintf("message is too large: %d", int(e))
}

type signedMessageTooSmallError int

func (e signedMessageTooSmallError) Error() string {
	return fmt.Sprintf("message is too small: %d", int(e))
}

func EncodeSignedMessage(msg any) (string, error) {
	var wr strings.Builder
	enc := base64.NewEncoder(base64.URLEncoding, &wr)
	signedCompressedJsonEncoder(msg)(enc)
	enc.Close()
	output := wr.String()
	if len(output) >= 64 {
		return "", signedMessageTooLargeError(len(output))
	}
	return output, nil
}

type Encoder interface {
	Encode(any) error
}
type Decoder interface {
	Decode(any) error
}

func basicMessageDecoder(rd io.Reader) Decoder {
	// return gob.NewDecoder(rd)
	return json.NewDecoder(rd)
}
func basicMessageEncoder(wr io.Writer) Encoder {
	// return gob.NewEncoder(wr)
	return json.NewEncoder(wr)
}

func signedCompressedJsonEncoder(msg any) func(wr io.Writer) error {
	return func(wr io.Writer) error {
		// hmac builder
		mac := hmac.New(sha256.New, secretSignature)
		// compressed json should be written to output and to hmac builder
		compressedJsonWriter := io.MultiWriter(wr, mac)

		gz := zlib.NewWriter(compressedJsonWriter)
		je := basicMessageEncoder(gz)
		if err := je.Encode(msg); err != nil {
			return err
		}
		if err := gz.Close(); err != nil {
			return err
		}
		// now we can append singature to the end of the message
		hmacHex := hex.EncodeToString(mac.Sum(nil))
		wr.Write([]byte(hmacHex)[:4])
		return nil
	}
}

func Verify(msg, hash []byte) (bool, error) {
	mac := hmac.New(sha256.New, secretSignature)
	mac.Write(msg)

	return hmac.Equal(
		hash,
		[]byte(hex.EncodeToString(mac.Sum(nil)))[:4],
	), nil
}
