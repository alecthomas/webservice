package main

import (
	"errors"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
)

var (
	pathTransform = regexp.MustCompile(`(<\w+>)`)
)

type RouteHandler func(args []string, route *Route, writer http.ResponseWriter, r *http.Request) bool

// Routes are specified like so:
//      /some/path/<arg0>/<arg1>
// arg0 and arg1 are mapped to handler method arguments.
type Route struct {
	pattern  *regexp.Regexp
	method   string
	params   []string
	function reflect.Value
	handler  RouteHandler
}

func NewRoute(path string, method string, function reflect.Value, handler RouteHandler) *Route {
	routePattern := "^" + pathTransform.ReplaceAllString(path, `([^/]+)`) + "$"
	pattern, _ := regexp.Compile(routePattern)
	route := &Route{
		pattern:  pattern,
		method:   method,
		function: function,
		handler:  handler,
	}
	for _, arg := range pathTransform.FindAllString(path, 16) {
		route.params = append(route.params, arg[1:len(arg)-1])
	}
	return route
}

func (r *Route) Match(req *http.Request) []string {
	if r.method != req.Method {
		return nil
	}
	return r.pattern.FindStringSubmatch(req.RequestURI)
}

func (r *Route) Apply(args []string, writer http.ResponseWriter, req *http.Request) bool {
	return r.handler(args[1:], r, writer, req)
}

type HttpService struct {
	Root     string
	Fallback http.Handler
	t        interface{}
	tt       reflect.Type
	tv       reflect.Value
	routes   []*Route
}

func NewHttpService(target interface{}, root string) *HttpService {
	return &HttpService{
		Root:     root,
		Fallback: http.NotFoundHandler(),
		t:        target,
		tt:       reflect.TypeOf(target),
		tv:       reflect.ValueOf(target),
	}
}

func (w *HttpService) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	for _, route := range w.routes {
		args := route.Match(req)
		if len(args) != 0 {
			if route.Apply(args, writer, req) {
				break
			}
		}
	}
	w.Fallback.ServeHTTP(writer, req)
}

func (w *HttpService) coerce(s string, t reflect.Type) (reflect.Value, error) {
	switch t.Kind() {
	case reflect.Int:
		v, err := strconv.ParseInt(s, 10, 64)
		return reflect.ValueOf(int(v)), err
	case reflect.Float32:
		v, err := strconv.ParseFloat(s, 32)
		return reflect.ValueOf(float32(v)), err
	case reflect.Float64:
		v, err := strconv.ParseFloat(s, 64)
		return reflect.ValueOf(v), err
	case reflect.String:
		return reflect.ValueOf(s), nil
	}
	return reflect.ValueOf(s), errors.New("unsupported argument type " + t.String())
}

func (w *HttpService) Get(path string, methodName string) error {
	return w.Route("GET", path, methodName)
}

func (w *HttpService) Post(path string, methodName string) error {
	return w.Route("POST", path, methodName)
}

func (w *HttpService) Put(path string, methodName string) error {
	return w.Route("PUT", path, methodName)
}

func (w *HttpService) Delete(path string, methodName string) error {
	return w.Route("DELETE", path, methodName)
}

func (w *HttpService) Route(method, path, methodName string) error {
	path = w.Root + path
	function := w.tv.MethodByName(methodName)
	if !function.IsValid() {
		return errors.New("unknown method " + methodName)
	}
	functype := function.Type()
	route := NewRoute(path, method, function, func(args []string, route *Route, writer http.ResponseWriter, request *http.Request) bool {
		in := make([]reflect.Value, 1, len(args))
		in[0] = reflect.ValueOf(&HttpServiceContext{writer, request})
		if functype.NumIn() != len(args)+1 {
			log.Println("invalid number of args")
			writer.WriteHeader(500)
			return true
		}
		for i, s := range args {
			v, err := w.coerce(s, functype.In(i+1))
			if err != nil {
				return false
			}
			in = append(in, v)
		}
		route.function.Call(in)
		return true
	})
	w.routes = append(w.routes, route)
	return nil
}

type HttpServiceContext struct {
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}

type MyService struct {
}

func (m *MyService) Create(cx *HttpServiceContext) {
	println("Create")
}

func (m *MyService) Read(cx *HttpServiceContext, id int) {
	println("Read", id)
}

func (m *MyService) Update(cx *HttpServiceContext, id int) {
	println("Update", id)
}

func (m *MyService) Delete(cx *HttpServiceContext, id int) {
	println("Delete", id)
}

func main() {
	ms := &MyService{}
	ws := NewHttpService(ms, "/blobstore/")
	ws.Post("", "Create")
	ws.Get("<id>", "Read")
	ws.Put("<id>", "Update")
	ws.Delete("<id>", "Deete")

	http.Handle("/blobstore/", ws)
	http.Handle("/", http.NotFoundHandler())
	http.ListenAndServe(":8080", nil)
}
