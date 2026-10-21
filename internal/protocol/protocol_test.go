// 文件说明：验证帧字节序、消息边界、残缺包拒绝与 protobuf 往返。
package protocol

import (
	"bytes"
	"encoding/binary"
	"game-shelf/pkg/contracts"
	"testing"
)

func TestFrameAndProtobufRoundTrip(t *testing.T) {
	source := &contracts.PingReq{UnixMillis: 12345}
	msg, err := MarshalMessage(77, 8, source)
	if err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	if err := WriteMessage(&wire, msg); err != nil {
		t.Fatal(err)
	}
	raw := append([]byte(nil), wire.Bytes()...)
	if binary.BigEndian.Uint32(raw[4:8]) != 77 || binary.BigEndian.Uint32(raw[8:12]) != 8 {
		t.Fatal("header byte order")
	}
	result, err := ReadMessage(bytes.NewReader(raw), 4096)
	if err != nil {
		t.Fatal(err)
	}
	var dest contracts.PingReq
	if err := UnmarshalBody(result, &dest); err != nil || dest.UnixMillis != 12345 {
		t.Fatal("protobuf round trip", err)
	}
	for _, cut := range []int{0, 1, 15, len(raw) - 1} {
		if _, err := ReadMessage(bytes.NewReader(raw[:cut]), 4096); err == nil {
			t.Fatal("truncated frame accepted", cut)
		}
	}
}
func TestBodySizeLimitBeforeAllocation(t *testing.T) {
	header := make([]byte, 16)
	binary.BigEndian.PutUint32(header, 1000000)
	if _, err := ReadMessage(bytes.NewReader(header), 4096); err == nil {
		t.Fatal("oversized body accepted")
	}
}
