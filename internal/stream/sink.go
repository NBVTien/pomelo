package stream

type Sink interface {
	SendJSON(v any) error
	SendJSONBytes(b []byte) error
	SendText(b []byte) error
	SendBinary(b []byte) error
	Close() error
}
