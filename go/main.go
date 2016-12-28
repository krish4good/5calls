package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/patrickmn/go-cache"
)

var (
	addr         = flag.String("addr", ":8090", "[ip]:port to listen on")
	dbfile       = flag.String("dbfile", "fivecalls.db", "filename for sqlite db")
	airtableBase = flag.String("airtable-base", "app6dzsa26hDjI7tp", "base ID for airtable store")
	airtableKey  = os.Getenv("AIRTABLE_API_KEY")
	civicKey     = os.Getenv("CIVIC_API_KEY")

	db         *sql.DB
	countCache = cache.New(1*time.Hour, 10*time.Minute)
)

var globalIssues IssueLister
var repFinder RepFinder

var pagetemplate *template.Template

func main() {
	flag.Parse()

	if airtableKey == "" {
		log.Fatal("No airtable API key found")
	}
	if civicKey == "" {
		log.Fatal("No google civic API key found")
	}

	atClient := NewAirtableClient(AirtableConfig{
		BaseID: *airtableBase,
		APIKey: airtableKey,
	})

	var gErr error
	globalIssues, gErr = NewIssueCache(atClient, 30*time.Minute)
	if gErr != nil {
		log.Fatalln(gErr)
	}

	cAPI := NewCivicAPI(civicKey, http.DefaultClient)
	repFinder = NewRepCache(cAPI, time.Hour, 10*time.Minute)

	// index template... unused
	p, err := template.ParseFiles("index.html")
	if err != nil {
		log.Println("can't parse template:", err)
	}
	pagetemplate = p

	// open database
	db, err = sql.Open("sqlite3", fmt.Sprintf("./%s", *dbfile))
	if err != nil {
		log.Printf("can't open databse: %s", err)
		return
	}
	defer db.Close()

	// set up http routing
	r := mux.NewRouter()
	r.HandleFunc("/issues/{zip}", pageHandler)
	r.HandleFunc("/issues/", pageHandler)
	r.HandleFunc("/admin/", adminHandler)
	r.HandleFunc("/admin/refresh", adminRefreshHandler)
	r.HandleFunc("/report/", reportHandler)
	http.Handle("/", r)
	log.Printf("running fivecalls-web on port %v", *addr)

	log.Fatal(http.ListenAndServe(*addr, nil))
}

func pageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET only", 403)
		return
	}

	vars := mux.Vars(r)
	zip := vars["zip"]

	var localReps *LocalReps
	var err error

	if len(zip) == 5 && zip != "" {
		log.Printf("zip %s", zip)
		localReps, _, err = repFinder.GetReps(zip)
		if err != nil {
			log.Println("Unable to find local reps for", zip, err)
		}
	} else {
		log.Printf("no zip")
	}

	// add local reps where necessary
	customizedIssues := []Issue{}
	all, err := globalIssues.AllIssues()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, issue := range all {
		newContacts := []Contact{}
		for _, contact := range issue.Contacts {
			if contact.Name == "LOCAL REP" {
				if localReps != nil {
					if localReps.HouseRep != nil {
						c := *localReps.HouseRep
						c.Reason = "This is your local representative in the house"
						newContacts = append(newContacts, c)
					}
					for _, s := range localReps.Senators {
						c := *s
						c.Reason = "This is one of your two senators"
						newContacts = append(newContacts, c)
					}
				}
			} else {
				newContacts = append(newContacts, contact)
			}
		}
		issue.Contacts = newContacts
		customizedIssues = append(customizedIssues, issue)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(customizedIssues)
}
