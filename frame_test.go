package wirenet

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/golang/protobuf/proto"
	wirenettest "github.com/mediabuyerbot/go-wirenet/testdata"

	"github.com/stretchr/testify/assert"
)

func genPayload(n int) []byte {
	b := make([]byte, n)
	var i int
	for i < n {
		b[i] = 'x'
		i++
	}
	return b
}

func TestFrame_EncodeDecode(t *testing.T) {
	stream := bytes.NewBuffer(nil)
	cmd := "test"
	payload := genPayload(1000)

	err := newEncoder(stream).Encode(initFrame, cmd, payload)
	assert.Nil(t, err)
	assert.Equal(t, headerLength+len(cmd)+len(payload), stream.Len())

	frm, err := newDecoder(stream).Decode()
	assert.Nil(t, err)

	assert.Equal(t, cmd, frm.Command())
	assert.Equal(t, len(cmd), frm.CommandLen())
	assert.Equal(t, len(payload), frm.PayloadLen())
	assert.Equal(t, payload, frm.Payload())
	assert.Equal(t, initFrame, frm.Type())
}

func BenchmarkFrame_DefaultEncodeDecode(b *testing.B) {
	stream := bytes.NewBuffer(nil)
	cmd := "test"
	payload := []byte("TEST")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := newEncoder(stream).Encode(initFrame, cmd, payload); err != nil {
			b.Fatal(err)
		}
		frm, err := newDecoder(stream).Decode()
		if err != nil {
			b.Fatal(err)
		}
		if frm.Command() != cmd {
			b.Fatalf("got %s expected %s",
				frm.Command(), cmd)
		}
	}
}

type jsonFrame struct {
	Type    uint32
	Cmd     string
	Payload []byte
}

func BenchmarkFrame_JSONEncodeDecode(b *testing.B) {
	stream := bytes.NewBuffer(nil)
	jf := jsonFrame{
		Type:    initFrame,
		Payload: []byte("TEST"),
		Cmd:     "test",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := json.NewEncoder(stream).Encode(jf); err != nil {
			b.Fatal(err)
		}
		var re jsonFrame
		if err := json.NewDecoder(stream).Decode(&re); err != nil {
			b.Fatal(err)
		}
		if re.Cmd != jf.Cmd {
			b.Fatalf("got %s expected %s",
				re.Cmd, jf.Cmd)
		}
	}
}

func BenchmarkFrame_PROTOEncodeDecode(b *testing.B) {
	stream := bytes.NewBuffer(nil)
	jf := wirenettest.Frame{
		Type:    initFrame,
		Cmd:     "test",
		Payload: []byte("TEST"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frm, err := proto.Marshal(&jf)
		if err != nil {
			b.Fatal(err)
		}
		stream.Write(frm)

		rd, err := ioutil.ReadAll(stream)
		if err != nil {
			b.Fatal(err)
		}
		var win wirenettest.Frame
		if err := proto.Unmarshal(rd, &win); err != nil {
			b.Fatal(err)
		}
		if win.Cmd != jf.Cmd {
			b.Fatalf("got %s expected %s",
				win.Cmd, jf.Cmd)
		}
	}
}
