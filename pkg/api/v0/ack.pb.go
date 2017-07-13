// Code generated by protoc-gen-go. DO NOT EDIT.
// source: ack.proto

package v0

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type Ack struct {
	Inbox    string `protobuf:"bytes,1,opt,name=inbox" json:"inbox,omitempty"`
	Subject  string `protobuf:"bytes,2,opt,name=subject" json:"subject,omitempty"`
	Sequence uint64 `protobuf:"varint,3,opt,name=sequence" json:"sequence,omitempty"`
}

func (m *Ack) Reset()                    { *m = Ack{} }
func (m *Ack) String() string            { return proto.CompactTextString(m) }
func (*Ack) ProtoMessage()               {}
func (*Ack) Descriptor() ([]byte, []int) { return fileDescriptor2, []int{0} }

func (m *Ack) GetInbox() string {
	if m != nil {
		return m.Inbox
	}
	return ""
}

func (m *Ack) GetSubject() string {
	if m != nil {
		return m.Subject
	}
	return ""
}

func (m *Ack) GetSequence() uint64 {
	if m != nil {
		return m.Sequence
	}
	return 0
}

func init() {
	proto.RegisterType((*Ack)(nil), "capsule8.v0.Ack")
}

func init() { proto.RegisterFile("ack.proto", fileDescriptor2) }

var fileDescriptor2 = []byte{
	// 121 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x4c, 0x4c, 0xce, 0xd6,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x4e, 0x4e, 0x2c, 0x28, 0x2e, 0xcd, 0x49, 0xb5, 0xd0,
	0x2b, 0x33, 0x50, 0x0a, 0xe4, 0x62, 0x76, 0x4c, 0xce, 0x16, 0x12, 0xe1, 0x62, 0xcd, 0xcc, 0x4b,
	0xca, 0xaf, 0x90, 0x60, 0x54, 0x60, 0xd4, 0xe0, 0x0c, 0x82, 0x70, 0x84, 0x24, 0xb8, 0xd8, 0x8b,
	0x4b, 0x93, 0xb2, 0x52, 0x93, 0x4b, 0x24, 0x98, 0xc0, 0xe2, 0x30, 0xae, 0x90, 0x14, 0x17, 0x47,
	0x71, 0x6a, 0x61, 0x69, 0x6a, 0x5e, 0x72, 0xaa, 0x04, 0xb3, 0x02, 0xa3, 0x06, 0x4b, 0x10, 0x9c,
	0xef, 0xc4, 0x12, 0xc5, 0x54, 0x66, 0x90, 0xc4, 0x06, 0xb6, 0xcc, 0x18, 0x10, 0x00, 0x00, 0xff,
	0xff, 0x82, 0xf9, 0x68, 0x7c, 0x79, 0x00, 0x00, 0x00,
}
