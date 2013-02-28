package webservice

import (
	"bytes"
	"fmt"
	"github.com/stretchrcom/testify/assert"
	"github.com/vmihailenco/msgpack"
	"net/http"
	"net/http/httptest"
	"testing"
)

type Req struct {
	Seen bool
}

func (r *Req) String() string {
	return fmt.Sprintf("Req{Seen: %v}", r.Seen)
}

func TestRequestDecoding(t *testing.T) {
	called := false
	handler := func(cx *Context, req *Req) {
		assert.True(t, req.Seen)
		called = true
	}
	r := NewRoute().Get().Path("/").DecodeRequest(&Req{}).ToFunction(handler)
	req, err := http.NewRequest("GET", "http://example.com/", bytes.NewReader([]byte(`{"Seen": true}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	assert.NoError(t, err)
	writer := httptest.NewRecorder()
	args := r.match(req)
	assert.Equal(t, len(args), 1)
	assert.True(t, r.apply(args, writer, req))
	assert.Equal(t, writer.Code, 200)
	assert.True(t, called)
}

func TestRequestDecodingMsgpack(t *testing.T) {
	called := false
	handler := func(cx *Context, req *Req) {
		assert.True(t, req.Seen)
		called = true
	}
	r := NewRoute().Get().Path("/").DecodeRequest(&Req{}).ToFunction(handler)
	rr := &Req{Seen: true}
	rrb, _ := msgpack.Marshal(rr)
	req, err := http.NewRequest("GET", "http://example.com/", bytes.NewReader(rrb))
	req.Header.Set("Content-Type", "application/x-msgpack")
	req.Header.Set("Accept", "application/x-msgpack")
	assert.NoError(t, err)
	writer := httptest.NewRecorder()
	args := r.match(req)
	assert.Equal(t, len(args), 1)
	assert.True(t, r.apply(args, writer, req))
	assert.Equal(t, writer.Code, 200)
	assert.True(t, called)
}
