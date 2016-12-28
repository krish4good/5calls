package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
	civicCache   = cache.New(1*time.Hour, 10*time.Minute)

	db         *sql.DB
	countCache = cache.New(1*time.Hour, 10*time.Minute)
)


var globalIssues IssueLister

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

	localContacts := []Contact{}

	if len(zip) == 5 && zip != "" {
		log.Printf("zip %s", zip)

		googResponse, err := getReps(zip)
		if err != nil {
			panic(err)
		}

		// remove president and vice president from officials
		validOfficials := []GoogleOfficial{}
		for _, office := range googResponse.Offices {
			if strings.Contains(office.Name, "President") {
				continue
			}

			for _, index := range office.OfficialIndices {
				official := googResponse.Officials[index]

				if strings.Contains(office.Name, "Senate") {
					official.Area = "Senate"
				} else if strings.Contains(office.Name, "House") {
					official.Area = "House"
				} else {
					official.Area = "Other"
				}

				validOfficials = append(validOfficials, official)
			}
		}

		for _, rep := range validOfficials {
			localContacts = append(localContacts, Contact{
				Name:     rep.Name,
				Phone:    rep.Phones[0],
				PhotoURL: rep.PhotoURL,
				Area:     rep.Area,
			})
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
		addContacts := false
		newContacts := []Contact{}

		for _, contact := range issue.Contacts {
			if contact.Name == "LOCAL REP" {
				addContacts = true
			} else {
				newContacts = append(newContacts, contact)
			}
		}

		if addContacts {
			// add the local contacts loaded from google civic
			for _, contact := range localContacts {
				if contact.Area == "Senate" {
					contact.Reason = "This is one of your two Senators"
				} else if contact.Area == "House" {
					contact.Reason = "This is your local representative in the House"
				}

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
