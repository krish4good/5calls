package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"
)

var pagetemplate *template.Template

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println("[WARN]", err)
	}
}

func enableCORS(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		fn(w, r)
	}
}

func main() {

	dbfile := flag.String("dbfile", "fivecalls.db", "filename for sqlite db")
	airtableBase := flag.String("airtable-base", "app6dzsa26hDjI7tp", "base ID for airtable store")
	addr := flag.String("addr", ":8090", "[ip]:port to listen on")
	airtableKey := os.Getenv("AIRTABLE_API_KEY")
	civicKey := os.Getenv("CIVIC_API_KEY")

	flag.Parse()

	if airtableKey == "" {
		log.Fatalln("No airtable API key found")
	}
	if civicKey == "" {
		log.Fatalln("No google civic API key found")
	}

	atClient := NewAirtableClient(AirtableConfig{
		BaseID: *airtableBase,
		APIKey: airtableKey,
	})

	issueLister, err := NewIssueCache(atClient, 30*time.Minute)
	if err != nil {
		log.Fatalln(err)
	}

	cAPI := NewCivicAPI(civicKey, http.DefaultClient)
	repFinder := NewRepCache(cAPI, time.Hour, 10*time.Minute)

	// index template... unused
	p, err := template.ParseFiles("index.html")
	if err != nil {
		log.Println("can't parse template:", err)
	}
	pagetemplate = p

	// open database
	file := fmt.Sprintf("./%s", *dbfile)
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		log.Fatalln("can't open database,", file, err)
	}
	defer db.Close()

	h := &handler{
		repFinder:   repFinder,
		issueLister: issueLister,
	}
	a := &adminHandler{
		reloader: issueLister.(cacheReloader),
	}
	rh := &reportHandler{db: db}
	// set up http routing
	r := mux.NewRouter()

	// all the GETs
	get := r.Methods("GET").Subrouter()
	get.HandleFunc("/issues/{zip}", enableCORS(h.GetIssues))
	get.HandleFunc("/issues/", enableCORS(h.GetIssues))
	get.HandleFunc("/admin/", a.Stats)
	get.HandleFunc("/report/", enableCORS(rh.Stats))
	get.HandleFunc("/report", enableCORS(rh.Stats))

	// all the POSTs
	post := r.Methods("POST").Subrouter()
	post.HandleFunc("/admin/refresh", a.ReloadCache)
	post.HandleFunc("/report", enableCORS(rh.RegisterCall))

	log.Printf("running fivecalls-web on port %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, r))
}
