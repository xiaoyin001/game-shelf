// 文件说明：实现统一 TCP 二进制协议的固定消息头读写和 protobuf 编解码。
package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/golang/protobuf/proto"
)

const HeaderSize = 16

type Message struct {
	BodyLen uint32
	ProtoID uint32
	Seq     uint32
	Flags   uint32
	Body    []byte
}

func ReadMessage(r io.Reader, maxBodyBytes int) (Message, error) {
	var header [HeaderSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Message{}, err
	}

	msg := Message{
		BodyLen: binary.BigEndian.Uint32(header[0:4]),
		ProtoID: binary.BigEndian.Uint32(header[4:8]),
		Seq:     binary.BigEndian.Uint32(header[8:12]),
		Flags:   binary.BigEndian.Uint32(header[12:16]),
	}
	if maxBodyBytes > 0 && int(msg.BodyLen) > maxBodyBytes {
		return Message{}, fmt.Errorf("body too large: %d > %d", msg.BodyLen, maxBodyBytes)
	}
	if msg.BodyLen == 0 {
		return msg, nil
	}
	msg.Body = make([]byte, msg.BodyLen)
	if _, err := io.ReadFull(r, msg.Body); err != nil {
		return Message{}, err
	}
	return msg, nil
}

func WriteMessage(w io.Writer, msg Message) error {
	if msg.BodyLen == 0 {
		msg.BodyLen = uint32(len(msg.Body))
	}
	var header [HeaderSize]byte
	binary.BigEndian.PutUint32(header[0:4], msg.BodyLen)
	binary.BigEndian.PutUint32(header[4:8], msg.ProtoID)
	binary.BigEndian.PutUint32(header[8:12], msg.Seq)
	binary.BigEndian.PutUint32(header[12:16], msg.Flags)
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if len(msg.Body) == 0 {
		return nil
	}
	_, err := w.Write(msg.Body)
	return err
}

func MarshalMessage(protoID, seq uint32, body proto.Message) (Message, error) {
	data, err := proto.Marshal(body)
	if err != nil {
		return Message{}, err
	}
	return Message{
		BodyLen: uint32(len(data)),
		ProtoID: protoID,
		Seq:     seq,
		Body:    data,
	}, nil
}

func UnmarshalBody(msg Message, body proto.Message) error {
	if len(msg.Body) == 0 {
		return nil
	}
	return proto.Unmarshal(msg.Body, body)
}

func SetDeadline(conn net.Conn, timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
}
