package inbox

import (
	"appengine"
	"appengine/datastore"
	"appengine/channel"
	"html/template"
	"inmail"
	"net/http"
	"strconv"
)

type TemplateData struct {
	Token    string
	Messages []inmail.Message
}

func init() {
	http.HandleFunc("/", getMessages)
	http.HandleFunc("/message", onMessage)
}

func onMessage(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		getMessages(w, req)
		break
	}
}

func getMessages(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	c := appengine.NewContext(req)
	q := datastore.NewQuery("Message").Order("-ReceivedDate").Limit(50)

	messages := make([]inmail.Message, 0, 50)
	if _, err := q.GetAll(c, &messages); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	low, _, err := datastore.AllocateIDs(c, "endpoint", nil, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	token, err := channel.Create(c, strconv.FormatInt(low, 10))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := inboxTemplate.Execute(w, TemplateData{token, messages}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var inboxTemplate = template.Must(template.New("inbox.html").ParseFiles("tmpl/inbox.html"))
