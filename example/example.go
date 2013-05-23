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
	ws := ws.NewService("/blobstore/")
	fmt.Printf("%s\n", ws.Post().Path("").ToMethod(ms, "Create"))
	fmt.Printf("%s\n", ws.Get().Path("{id}").ToMethod(ms, "Read"))
	fmt.Printf("%s\n", ws.Put().Path("{id}").ToMethod(ms, "Update"))
	fmt.Printf("%s\n", ws.Delete().Path("{id}").ToMethod(ms, "Delete"))

	http.Handle("/", ws)
	http.ListenAndServe(":8080", nil)
}
