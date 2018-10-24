package main

import (
	"fmt"
	"net/http"

	"lenslockedbr.com/views"
	"github.com/gorilla/mux"
)

var homeView *views.View
var contactView *views.View

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	must(homeView.Render(w, nil))
}

func contact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	must(contactView.Render(w, nil))
}

func faq(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, "<h1>F.A.Q.</h1>" +
                      "<p>You'll find here some answers which could be " +
		      "helpful for you.</p>" +
		      "<p><ol><li>Foobar</li>" +
                      "<li>XPTO</li>" +
                      "<li>Foo</li>" +
		      "<li>Bar</li></ol></p>" +
                      "<p><a href=\"/\">Home</a></p>")
}

func notfound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type","text/html")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, "<h2>WHOW! Page Not Found - 404!</h2>")
}

// A helper function that panics on any error
func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	homeView = views.NewView("bootstrap", "views/home.gohtml")
	contactView = views.NewView("bootstrap", "views/contact.gohtml")

	r := mux.NewRouter()

	r.NotFoundHandler = http.HandlerFunc(notfound)

	r.HandleFunc("/", home)
	r.HandleFunc("/contact", contact)
	r.HandleFunc("/faq", faq)
	http.ListenAndServe(":3000", r)
}
