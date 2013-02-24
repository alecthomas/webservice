package main

import (
	"fmt"
	ws "github.com/alecthomas/webservice"
	"net/http"
)

type MyService struct {
}

func (m *MyService) Create(cx *ws.Context) {
	println("Create")
}

func (m *MyService) Read(cx *ws.Context, id int) {
	println("Read", id)
}

func (m *MyService) Update(cx *ws.Context, id int) {
	println("Update", id)
}

func (m *MyService) Delete(cx *ws.Context, id int) {
	println("Delete", id)
}

func main() {
	ms := &MyService{}
	ws := ws.NewService()
	fmt.Printf("%s\n", ws.Post().Path("/blobstore/").DispatchToMethod(ms, "Create"))
	ws.Get().Path("/blobstore/{id}").DispatchToMethod(ms, "Read")
	ws.Put().Path("/blobstore/{id}").DispatchToMethod(ms, "Update")
	ws.Delete().Path("/blobstore/{id}").DispatchToMethod(ms, "Delete")

	http.Handle("/blobstore/", ws)
	http.Handle("/", ws.FallbackHandler)
	http.ListenAndServe(":8080", nil)
}
