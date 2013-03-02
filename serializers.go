package webservice

import (
	"code.google.com/p/vitess/go/bson"
	"encoding/json"
	"errors"
	"github.com/vmihailenco/msgpack"
	"io"
	"net/http"
)

var (
	Serializers = SerializerMap{
		"application/json":      &JsonSerializer{},
		"application/x-msgpack": &MsgpackSerializer{},
		"application/bson":      &BsonSerializer{},
	}
	UnsupportedContentType = errors.New("unsupported content type")
)

type ContentTypeDecoder interface {
	Decode(v interface{}) error
}

type ContentTypeEncoder interface {
	Encode(v interface{}) error
}

type Serializer interface {
	NewEncoder(w io.Writer) ContentTypeEncoder
	NewDecoder(r io.Reader) ContentTypeDecoder
}

type JsonSerializer struct{}

func (j *JsonSerializer) NewEncoder(w io.Writer) ContentTypeEncoder {
	return json.NewEncoder(w)
}

func (j *JsonSerializer) NewDecoder(r io.Reader) ContentTypeDecoder {
	return json.NewDecoder(r)
}

type MsgpackSerializer struct{}

func (j *MsgpackSerializer) NewEncoder(w io.Writer) ContentTypeEncoder {
	return msgpack.NewEncoder(w)
}

func (j *MsgpackSerializer) NewDecoder(r io.Reader) ContentTypeDecoder {
	return msgpack.NewDecoder(r)
}

type BsonSerializer struct{}

type bsonEncoder struct {
	w io.Writer
}

func (b *bsonEncoder) Encode(v interface{}) error {
	return bson.MarshalToStream(b.w, v)
}

func (j *BsonSerializer) NewEncoder(w io.Writer) ContentTypeEncoder {
	return &bsonEncoder{w}
}

type bsonDecoder struct {
	r io.Reader
}

func (b *bsonDecoder) Decode(v interface{}) error {
	return bson.UnmarshalFromStream(b.r, v)
}

func (j *BsonSerializer) NewDecoder(r io.Reader) ContentTypeDecoder {
	return &bsonDecoder{r}
}

type SerializerMap map[string]Serializer

func (s SerializerMap) Decode(req *http.Request, v interface{}) error {
	ct := req.Header.Get("Content-Type")
	if ser, ok := s[ct]; ok {
		decoder := ser.NewDecoder(req.Body)
		return decoder.Decode(v)
	}
	return UnsupportedContentType
}

func (s SerializerMap) Encode(req *http.Request, resp http.ResponseWriter, response *Response) error {
	ct := req.Header.Get("Accept")
	if ct == "" {
		ct = req.Header.Get("Content-Type")
	}
	resp.Header().Set("Content-Type", ct)
	if ser, ok := s[ct]; ok {
		encoder := ser.NewEncoder(resp)
		resp.WriteHeader(response.S)
		return encoder.Encode(response)
	}
	resp.WriteHeader(http.StatusBadRequest)
	return UnsupportedContentType
}
