package srpc

// Message is the vtprotobuf message interface.
type Message interface {
	MarshalVT() ([]byte, error)
	UnmarshalVT([]byte) error
}

// RawMessage is a raw protobuf message container.
type RawMessage struct {
	data []byte
	copy bool
}

// NewRawMessage constructs a new raw message.
// If copy=true, copies data in MarshalVT.
// Note: the data buffer will be retained and used.
// The data buffer will be written to and/or replaced in UnmarshalVT.
func NewRawMessage(data []byte, copy bool) *RawMessage {
	return &RawMessage{data: data, copy: copy}
}

// GetData returns the data buffer without copying.
func (m *RawMessage) GetData() []byte {
	return m.data
}

// SetData sets the data buffer.
// if copy=true, copies the data to the internal slice.
// otherwise retains the buffer.
func (m *RawMessage) SetData(data []byte) {
	if m.copy {
		if cap(m.data) >= len(data) {
			m.data = m.data[:len(data)]
		} else {
			m.data = make([]byte, len(data))
		}
		copy(m.data, data)
	} else {
		m.data = data
	}
}

func (m *RawMessage) MarshalVT() ([]byte, error) {
	if !m.copy {
		return m.data, nil
	}

	data := make([]byte, len(m.data))
	copy(data, m.data)
	return data, nil
}

func (m *RawMessage) UnmarshalVT(data []byte) error {
	m.SetData(data)
	return nil
}

// _ is a type assertion
var _ Message = ((*RawMessage)(nil))
