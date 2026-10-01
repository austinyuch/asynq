package proto_test

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"io"
	"math/rand"
	"testing"
	"testing/quick"

	pb "github.com/austinyuch/asynq/internal/proto"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The fork's generated output must retain its wire names while advertising the
// fork Go package, including the legacy descriptor API used by v1 consumers.
func TestLegacyDescriptorAndTextInteroperability(t *testing.T) {
	messages := []proto.Message{
		&pb.TaskMessage{Type: "compat(task)", Id: "task", Queue: "critical", Payload: []byte{0, 255}, Headers: map[string]string{"locale": "zh-TW"}},
		&pb.ServerInfo{Host: "worker", Pid: 42, ServerId: "server", Queues: map[string]int32{"critical": 6}},
		&pb.WorkerInfo{Host: "worker", TaskId: "task", StartTime: &timestamppb.Timestamp{Seconds: 123}},
		&pb.SchedulerEntry{Id: "entry", TaskType: "scheduled", EnqueueOptions: []string{`Queue("a)b")`}},
		&pb.SchedulerEnqueueEvent{TaskId: "task", EnqueueTime: &timestamppb.Timestamp{Seconds: 456}},
	}
	for index, message := range messages {
		t.Run(string(message.ProtoReflect().Descriptor().Name()), func(t *testing.T) {
			legacy := protoadapt.MessageV1Of(message)
			legacy.ProtoMessage()
			text := legacy.String()
			decoded := message.ProtoReflect().Type().New().Interface()
			if err := prototext.Unmarshal([]byte(text), decoded); err != nil || !proto.Equal(message, decoded) {
				t.Fatalf("legacy text lost wire values: error=%v text=%s", err, text)
			}
			compressed, path := legacy.(interface{ Descriptor() ([]byte, []int) }).Descriptor()
			if len(path) != 1 || path[0] != index {
				t.Fatalf("legacy message index %v, want %d", path, index)
			}
			reader, err := gzip.NewReader(bytes.NewReader(compressed))
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			var descriptor descriptorpb.FileDescriptorProto
			if err := proto.Unmarshal(data, &descriptor); err != nil {
				t.Fatal(err)
			}
			if descriptor.GetPackage() != "asynq" || descriptor.GetOptions().GetGoPackage() != "github.com/austinyuch/asynq/internal/proto" {
				t.Fatalf("fork descriptor identity regressed: %v", &descriptor)
			}
			if descriptor.GetMessageType()[index].GetName() != string(message.ProtoReflect().Descriptor().Name()) {
				t.Fatal("legacy descriptor index resolves a different message")
			}
			registered, err := protoregistry.GlobalTypes.FindMessageByName(message.ProtoReflect().Descriptor().FullName())
			if err != nil || registered.Descriptor() != message.ProtoReflect().Descriptor() {
				t.Fatalf("wire registry identity mismatch: %v", err)
			}
			legacy.Reset()
			if !proto.Equal(message, message.ProtoReflect().Type().New().Interface()) {
				t.Fatal("legacy reset retained serialized data")
			}
		})
	}
}

func TestTypedNilLegacyReflection(t *testing.T) {
	for _, message := range []proto.Message{(*pb.TaskMessage)(nil), (*pb.ServerInfo)(nil), (*pb.WorkerInfo)(nil), (*pb.SchedulerEntry)(nil), (*pb.SchedulerEnqueueEvent)(nil)} {
		if message.ProtoReflect().IsValid() || protoadapt.MessageV2Of(protoadapt.MessageV1Of(message)).ProtoReflect().IsValid() {
			t.Fatalf("typed nil became a valid mutable message: %T", message)
		}
	}
}

// This fixed wire fixture is independent of the generated descriptor: changing
// field numbers or map encoding must not silently break persisted task messages.
func TestTaskWireGolden(t *testing.T) {
	golden, err := hex.DecodeString("0a01781a0269642201717a060a0161120162")
	if err != nil {
		t.Fatal(err)
	}
	want := &pb.TaskMessage{Type: "x", Id: "id", Queue: "q", Headers: map[string]string{"a": "b"}}
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(want)
	if err != nil || !bytes.Equal(encoded, golden) {
		t.Fatalf("wire encoding drifted: %x (%v)", encoded, err)
	}
	var decoded pb.TaskMessage
	if err := proto.Unmarshal(golden, &decoded); err != nil || !proto.Equal(want, &decoded) {
		t.Fatalf("persisted golden cannot be read: %v", err)
	}
}

func taskWireUnknownFieldProperty(payload []byte, retry int32) bool {
	message := &pb.TaskMessage{Type: "wire", Queue: "q", Payload: payload, Retry: retry}
	data, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
	if err != nil {
		return false
	}
	unknown := protowire.AppendTag(nil, 19000, protowire.VarintType)
	unknown = protowire.AppendVarint(unknown, 42)
	var decoded pb.TaskMessage
	if err := proto.Unmarshal(append(data, unknown...), &decoded); err != nil {
		return false
	}
	if !bytes.Equal(decoded.ProtoReflect().GetUnknown(), unknown) {
		return false
	}
	decoded.ProtoReflect().SetUnknown(nil)
	return proto.Equal(message, &decoded)
}

func TestTaskWireUnknownFieldProperty(t *testing.T) {
	if err := quick.Check(taskWireUnknownFieldProperty, &quick.Config{MaxCount: 10000, Rand: rand.New(rand.NewSource(20261001))}); err != nil {
		t.Fatal(err)
	}
}

func FuzzTaskWireUnknownFields(f *testing.F) {
	f.Add([]byte{0, 255}, int32(-1))
	f.Add([]byte{}, int32(0))
	f.Fuzz(func(t *testing.T, payload []byte, retry int32) {
		if len(payload) > 4096 {
			t.Skip("bounded wire contract payload")
		}
		if !taskWireUnknownFieldProperty(payload, retry) {
			t.Fatal("payload or unknown field compatibility regressed")
		}
	})
}
