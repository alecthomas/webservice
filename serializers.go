package webservice

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/vmihailenco/msgpack"
	"io"
	"io/ioutil"
	"labix.org/v2/mgo/bson"
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

type Base64Encoder struct {
	w io.Writer
	e func(io.Writer) ContentTypeEncoder
}

func (m *Base64Encoder) Encode(v interface{}) error {
	b := base64.NewEncoder(base64.StdEncoding, m.w)
	defer b.Close()
	return m.e(b).Encode(v)
}

type Base64Decoder struct {
	r io.Reader
	d func(io.Reader) ContentTypeDecoder
}

func (b *Base64Decoder) Decode(v interface{}) error {
	b64 := base64.NewDecoder(base64.StdEncoding, b.r)
	return b.d(b64).Decode(v)
}

type MsgpackSerializer struct{}

func (j *MsgpackSerializer) NewEncoder(w io.Writer) ContentTypeEncoder {
	return &Base64Encoder{
		w: w,
		e: func(w io.Writer) ContentTypeEncoder { return msgpack.NewEncoder(w) },
	}
}

func (j *MsgpackSerializer) NewDecoder(r io.Reader) ContentTypeDecoder {
	return &Base64Decoder{
		r: r,
		d: func(r io.Reader) ContentTypeDecoder { return msgpack.NewDecoder(r) },
	}
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
	return &Base64Encoder{
		w: w,
		e: func(w io.Writer) ContentTypeEncoder { return &bsonEncoder{w} },
	}
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
	return &Base64Decoder{
		r: r,
		d: func(r io.Reader) ContentTypeDecoder { return &bsonDecoder{r} },
	}
}

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
	err := s.Encode(ct, resp, response)
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		return UnsupportedContentType
	}
	resp.WriteHeader(response.S)
	return nil
}

func (s SerializerMap) Encode(ct string, w io.Writer, v interface{}) error {
	if ser, ok := s[ct]; ok {
		encoder := ser.NewEncoder(w)
		return encoder.Encode(v)
	}
	return UnsupportedContentType
}
