package webservice

import (
	"code.google.com/p/vitess/go/bson"
	"encoding/json"
	"errors"
	"github.com/vmihailenco/msgpack"
	"io"
	"io/ioutil"
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

type SerializerMap map[string]Serializer

func (s SerializerMap) DecodeRequest(req *http.Request, v interface{}) error {
	ct := req.Header.Get("Content-Type")
	return s.Decode(ct, req.Body, v)
}

func (s SerializerMap) Decode(ct string, r io.Reader, v interface{}) error {
	if ser, ok := s[ct]; ok {
		decoder := ser.NewDecoder(r)
		return decoder.Decode(v)
	}
	return UnsupportedContentType
}

func (s SerializerMap) EncodeResponse(req *http.Request, resp http.ResponseWriter, response *Response) error {
	ct := req.Header.Get("Accept")
	if ct == "" {
		ct = req.Header.Get("Content-Type")
	}
	resp.Header().Set("Content-Type", ct)
	// TODO: Figure out ordering here that isn't shit.
	if ser, ok := s[ct]; ok {
		resp.WriteHeader(response.S)
		return s.rawEncode(ser, resp, response)
	}
	resp.WriteHeader(http.StatusBadRequest)
	return UnsupportedContentType
}

func (s SerializerMap) Encode(ct string, w io.Writer, v interface{}) error {
	if ser, ok := s[ct]; ok {
		return s.rawEncode(ser, w, v)
	}
	return UnsupportedContentType
}

func (s SerializerMap) rawEncode(ser Serializer, w io.Writer, v interface{}) error {
	encoder := ser.NewEncoder(w)
	return encoder.Encode(v)
}

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
	bytes, err := bson.Marshal(v)
	if err != nil {
		return err
	}
	_, err = b.w.Write(bytes)
	return err
}

func (j *BsonSerializer) NewEncoder(w io.Writer) ContentTypeEncoder {
	return &bsonEncoder{w}
}

type bsonDecoder struct {
	r io.Reader
}

func (b *bsonDecoder) Decode(v interface{}) error {
	bytes, err := ioutil.ReadAll(b.r)
	if err != nil {
		return err
	}
	return bson.Unmarshal(bytes, v)
}

func (j *BsonSerializer) NewDecoder(r io.Reader) ContentTypeDecoder {
	return &bsonDecoder{r}
}
