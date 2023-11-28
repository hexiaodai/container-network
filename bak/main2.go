package main

import (
	"container-network/container"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

var (
	dbPath = "config.yaml"
)

func main() {
	router := httprouter.New()
	router.POST("/container/:name", containerHandler)

	fmt.Println("Listening :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func init() {

}

func containerHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_, err := container.NewContainer(ps.ByName("name"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}
