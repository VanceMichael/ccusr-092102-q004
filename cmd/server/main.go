package main

import (
	"log"
	"net/http"

	curriculum "example.com/teacher-curriculum-evidence"
)

func main() {
	handler := curriculum.NewHandler(curriculum.NewService())
	log.Fatal(http.ListenAndServe(":8080", handler))
}
