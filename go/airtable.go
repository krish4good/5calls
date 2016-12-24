package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

var issuesTable = "Issues%20list"
var contactsTable= "Contact"

func refreshIssuesAndContacts() {
	globalIssues, _ = fetchIssues()
	log.Printf("issues %v", globalIssues)
	contacts, _ := fetchContacts()
	log.Printf("contacts %v", contacts)

	// remove empty ones from airtable
	for index, issue := range globalIssues.Records {
		if issue.Fields.Name == "" {
			globalIssues.Records = append(globalIssues.Records[:index], globalIssues.Records[index+1:]...)
		}
	}

	// add contacts to the right spot
	for index, issue := range globalIssues.Records {
		if issue.Contacts == nil {
			issue.Contacts = []AirtableContact{}
		}

		for _, contactID := range issue.Fields.ContactIDs {
			for _, contact := range contacts.Records {
				if contactID == contact.ID {
					globalIssues.Records[index].Contacts = append(globalIssues.Records[index].Contacts, contact)
				}
			}
		}
	}

}

type AirtableIssues struct {
	Records []struct {
		ID     string `json:"id"`
		Fields struct {
			Name       string
			Action     string `json:"Action requested"`
			Script     string
			ContactIDs []string `json:"Contact"`
		}
		Contacts []AirtableContact
	} `json:"records"`
}

func (i AirtableIssues) exportIssues() []Issue {
	issues := []Issue{}
	// jsonData, _ := json.Marshal(i)
	// log.Printf("airtable: %s", jsonData)

	for _, airtableIssue := range i.Records {
		newIssue := Issue{ID: airtableIssue.ID, Name: airtableIssue.Fields.Name, Reason: airtableIssue.Fields.Action, Script: airtableIssue.Fields.Script}

		newContacts := []Contact{}
		for _, airtableContact := range airtableIssue.Contacts {
			newContact := Contact{
				Name:     airtableContact.Fields.Name,
				Phone:    airtableContact.Fields.Phone,
				PhotoURL: airtableContact.Fields.PhotoURL,
				Reason:   airtableContact.Fields.Reason,
				Area:     airtableContact.Fields.Area,
			}

			newContacts = append(newContacts, newContact)
		}
		newIssue.Contacts = newContacts

		issues = append(issues, newIssue)
	}

	return issues
}

func fetchIssues() (AirtableIssues, error) {
	formula := url.QueryEscape("NOT({Inactive})")
	url := fmt.Sprintf("%s/%s?filterByFormula=%s", *airtableUrl, issuesTable, formula)

	client := http.DefaultClient
	req, e := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", airtableKey))
	resp, e := client.Do(req)
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	parsedResponse := AirtableIssues{}
	if resp.StatusCode >= 200 && resp.StatusCode <= 400 && e == nil {
		json.Unmarshal(body, &parsedResponse)
		return parsedResponse, nil
	}

	return parsedResponse, fmt.Errorf("issue error code:%d error:%v body:%s", resp.StatusCode, e, body)
}

type AirtableContacts struct {
	Records []AirtableContact `json:"records"`
}

type AirtableContact struct {
	ID     string
	Fields struct {
		Name     string
		Phone    string
		PhotoURL string
		Area     string
		Reason   string `json:"Contact Reason"`
	}
}

func fetchContacts() (AirtableContacts, error) {
	url := fmt.Sprintf("%s/%s", *airtableUrl, contactsTable)

	client := http.DefaultClient
	req, e := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", airtableKey))
	resp, e := client.Do(req)
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	parsedResponse := AirtableContacts{}
	if resp.StatusCode >= 200 && resp.StatusCode <= 400 && e == nil {
		json.Unmarshal(body, &parsedResponse)
		return parsedResponse, nil
	}

	return parsedResponse, fmt.Errorf("contact error code:%d error:%v body:%s", resp.StatusCode, e, body)
}
