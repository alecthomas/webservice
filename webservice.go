package webservice

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var (
	pathTransform = regexp.MustCompile(`{((\w+)(\.\.\.)?)}`)
)

type Dispatcher func(cx *Context, req interface{}) bool
type Args map[string]string

type Response struct {
	S int
	E interface{} // always a string, but an interface{} here so it can be nil
	D interface{}
}

func FunctionDispatcher(function reflect.Value) Dispatcher {
	functype := function.Type()
	return func(cx *Context, req interface{}) bool {
		shift := 1
		if req != nil {
			shift++
		}
		if functype.NumIn() != shift+len(cx.Args) {
			cx.Respond(http.StatusInternalServerError, "invalid number of arguments", nil)
			return true
		}
		in := make([]reflect.Value, shift, shift+len(cx.Args))
		in[0] = reflect.ValueOf(cx)
		if req != nil {
			in[1] = reflect.ValueOf(req)
		}
		for i, s := range cx.Args {
			v, err := coerce(s, functype.In(i+shift))
			if err != nil {
				return false
			}
			in = append(in, v)
		}
		function.Call(in)
		return true
	}
}

// Routes are specified like so:
//      /some/path/{arg0}/{arg1}
// arg0 and arg1 are mapped to handler method arguments.
type Route struct {
	prefix  string
	path    string
	name    string
	pattern *regexp.Regexp
	methods []string
	params  []string
	handler Dispatcher
	request reflect.Type
}

func NewRoute() *Route {
	return &Route{}
}

func (r *Route) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	r.apply(r.match(req), writer, req)
}

func (r *Route) String() string {
	return fmt.Sprintf("Route{name: %v, pattern: %s, methods: %v}", r.name, r.pattern, r.methods)
}

func (r *Route) Reverse(args Args) string {
	path := r.fullPath()
	for arg, value := range args {
		path = strings.Replace(path, "{"+arg+"}", value, 1)
	}
	return path
}

func (r *Route) Method() string {
	if len(r.methods) != 1 {
		panic("requested single method from route with less or more")
	}
	return r.methods[0]
}

// Automatically decode the request body into a value of type req. Functions
// and methods will then be called with the signature
// f(*Context, TypeOf(req), ...)
func (r *Route) DecodeRequest(req interface{}) *Route {
	r.request = reflect.TypeOf(req)
	if r.request.Kind() != reflect.Ptr {
		panic("request structure must be a pointer")
	}
	return r
}

func (r *Route) Prefix(path string) *Route {
	r.prefix = strings.TrimRight(path, "/") + "/"
	return r.compilePath()
}

func (r *Route) Named(name string) *Route {
	r.name = name
	return r
}

func (r *Route) ToHandler(handler http.Handler) *Route {
	r.handler = func(cx *Context, req interface{}) bool {
		handler.ServeHTTP(cx.ResponseWriter, cx.Request)
		return true
	}
	return r
}

func (r *Route) ToHandlerFunc(handler http.HandlerFunc) *Route {
	r.handler = func(cx *Context, req interface{}) bool {
		handler(cx.ResponseWriter, cx.Request)
		return true
	}
	return r
}

func (r *Route) ToFunction(f interface{}) *Route {
	function := reflect.ValueOf(f)
	if function.Kind() != reflect.Func || !function.IsValid() {
		panic("invalid function")
	}
	r.handler = FunctionDispatcher(function)
	return r
}

func (r *Route) ToMethod(v interface{}, method string) *Route {
	rv := reflect.ValueOf(v)
	function := rv.MethodByName(method)
	if !function.IsValid() {
		panic("unknown method " + method)
	}
	r.handler = FunctionDispatcher(function)
	if r.name == "" {
		r.name = method
	}
	return r
}

func (r *Route) Path(path string) *Route {
	r.path = strings.TrimLeft(path, "/")
	return r.compilePath()
}

func (r *Route) compilePath() *Route {
	routePattern := "^" + r.fullPath() + "$"
	for _, match := range pathTransform.FindAllStringSubmatch(routePattern, 16) {
		pattern := `([^/]+)`
		if match[3] == "..." {
			pattern = `(.+)`
		}
		routePattern = strings.Replace(routePattern, match[0], pattern, 1)
	}
	pattern, _ := regexp.Compile(routePattern)
	r.pattern = pattern
	r.params = []string{}
	for _, arg := range pathTransform.FindAllString(r.prefix+r.path, 16) {
		r.params = append(r.params, arg[1:len(arg)-1])
	}
	return r
}

func (r *Route) Get() *Route {
	r.methods = append(r.methods, "GET")
	return r
}

func (r *Route) Delete() *Route {
	r.methods = append(r.methods, "DELETE")
	return r
}

func (r *Route) Put() *Route {
	r.methods = append(r.methods, "PUT")
	return r
}

func (r *Route) Post() *Route {
	r.methods = append(r.methods, "POST")
	return r
}

func (r *Route) match(req *http.Request) []string {
	if len(r.methods) != 0 {
		matchedMethod := false
		for _, m := range r.methods {
			if m == req.Method {
				matchedMethod = true
				break
			}
		}
		if !matchedMethod {
			return nil
		}
	}
	if r.pattern == nil {
		return []string{req.RequestURI}
	}
	return r.pattern.FindStringSubmatch(req.RequestURI)
}

func (r *Route) fullPath() string {
	return r.prefix + r.path
}

func (r *Route) apply(args []string, writer http.ResponseWriter, req *http.Request) bool {
	cx := &Context{args[1:], writer, req}
	defer cx.Request.Body.Close()
	var request interface{} = nil
	if r.request != nil {
		v := reflect.New(r.request.Elem())
		err := Serializers.DecodeRequest(req, v.Interface())
		if err != nil {
			cx.RespondWithErrorMessage(err.Error(), http.StatusBadRequest)
			return true
		}
		request = v.Interface()
	}
	return r.handler(cx, request)
}

type NotFoundHandler struct{}

func (n *NotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cx := &Context{ResponseWriter: w, Request: r}
	cx.RespondWithStatus(http.StatusNotFound)
}

type Service struct {
	Root            string
	FallbackHandler http.Handler
	routes          []*Route
}

func NewService(root string) *Service {
	return &Service{
		Root:            root,
		FallbackHandler: &NotFoundHandler{},
	}
}

func (s *Service) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	for _, route := range s.routes {
		args := route.match(req)
		if len(args) != 0 {
			if route.apply(args, writer, req) {
				return
			}
		}
	}
	s.FallbackHandler.ServeHTTP(writer, req)
}

func (s *Service) Find(name string) *Route {
	for _, r := range s.routes {
		if r.name == name {
			return r
		}
	}
	return nil
}

func (s *Service) route() *Route {
	route := NewRoute()
	if s.Root != "" {
		route.Prefix(s.Root)
	}
	s.routes = append(s.routes, route)
	return route
}

func (s *Service) Get() *Route {
	return s.route().Get()
}

func (s *Service) Put() *Route {
	return s.route().Put()
}

func (s *Service) Post() *Route {
	return s.route().Post()
}

func (s *Service) Delete() *Route {
	return s.route().Delete()
}

func (s *Service) Path(path string) *Route {
	return s.route().Path(path)
}

func (s *Service) ToFunction(f interface{}) *Route {
	return s.route().ToFunction(f)
}
func (s *Service) ToMethod(v interface{}, method string) *Route {
	return s.route().ToMethod(v, method)
}

func (s *Service) ToHandler(handler http.Handler) *Route {
	return s.route().ToHandler(handler)
}

func (s *Service) ToHandlerFunc(handler http.HandlerFunc) *Route {
	return s.route().ToHandlerFunc(handler)
}

func (s *Service) Named(name string) *Route {
	return s.route().Named(name)
}

type Context struct {
	Args           []string
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}

func (c *Context) Respond(status int, error string, data interface{}) error {
	var E interface{} = nil
	if error != "" {
		E = error
	}
	return Serializers.EncodeResponse(c.Request, c.ResponseWriter, &Response{S: status, E: E, D: data})
}

func (c *Context) RespondWithErrorMessage(error string, status int) error {
	return c.Respond(status, error, nil)
}

func (c *Context) Receive(v interface{}) error {
	return Serializers.DecodeRequest(c.Request, v)
}

func (c *Context) RespondWithStatus(status int) error {
	return c.Respond(status, "", nil)
}

func (c *Context) RespondWithData(v interface{}) error {
	return c.Respond(200, "", v)
}

func coerce(s string, t reflect.Type) (reflect.Value, error) {
	switch t.Kind() {
	case reflect.Int:
		v, err := strconv.ParseInt(s, 10, 64)
		return reflect.ValueOf(int(v)), err
	case reflect.Int8:
		v, err := strconv.ParseInt(s, 10, 8)
		return reflect.ValueOf(int8(v)), err
	case reflect.Uint8:
		v, err := strconv.ParseUint(s, 10, 8)
		return reflect.ValueOf(uint8(v)), err
	case reflect.Int16:
		v, err := strconv.ParseInt(s, 10, 16)
		return reflect.ValueOf(int16(v)), err
	case reflect.Uint16:
		v, err := strconv.ParseUint(s, 10, 16)
		return reflect.ValueOf(uint16(v)), err
	case reflect.Int32:
		v, err := strconv.ParseInt(s, 10, 32)
		return reflect.ValueOf(int32(v)), err
	case reflect.Uint32:
		v, err := strconv.ParseUint(s, 10, 32)
		return reflect.ValueOf(uint32(v)), err
	case reflect.Int64:
		v, err := strconv.ParseInt(s, 10, 64)
		return reflect.ValueOf(int64(v)), err
	case reflect.Uint64:
		v, err := strconv.ParseUint(s, 10, 64)
		return reflect.ValueOf(uint64(v)), err
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
