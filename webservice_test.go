package webservice

import (
	"bytes"
	"fmt"
	"github.com/stretchrcom/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

type Req struct {
	Seen    bool
	Message string
}

func (r *Req) String() string {
	return fmt.Sprintf("Req{Seen: %v}", r.Seen)
}

func testEncoderDecoder(t *testing.T, ct string) {
	called := false
	handler := func(cx *Context, req *Req) {
		assert.True(t, req.Seen)
		assert.Equal(t, req.Message, "hello")
		called = true
	}
	r := NewRoute().Get().Path("/").DecodeRequest(&Req{}).ToFunction(handler)
	rr := &Req{Seen: true, Message: "hello"}
	rrbw := &bytes.Buffer{}
	err := Serializers.Encode(ct, rrbw, rr)
	assert.NoError(t, err)
	req, err := http.NewRequest("GET", "http://example.com/", rrbw)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Accept", ct)
	assert.NoError(t, err)
	writer := httptest.NewRecorder()
	args := r.match(req)
	assert.Equal(t, len(args), 1)
	assert.True(t, r.apply(args, writer, req))
	assert.Equal(t, writer.Code, 200)
	assert.True(t, called)
}

func TestRequestDecodingMsgpack(t *testing.T) {
	testEncoderDecoder(t, "application/x-msgpack")
}

func TestRequestDecodingBson(t *testing.T) {
	testEncoderDecoder(t, "application/bson")
}

func TestRequestDecodingJson(t *testing.T) {
	testEncoderDecoder(t, "application/json")
}
