package inbox

import (
	"appengine"
	"appengine/datastore"
	"appengine/channel"
	"html/template"
	"inmail"
	"net/http"
	"strconv"
	"time"
)

type templateData struct {
	Token    string
	Messages []inboxItem
}

type inboxItem struct {
	Key				int64
	From  			string
	Subject 			string
	Body				template.HTML
	ImageUrls		*[]string
	ReceivedDate	time.Time
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
	keys, err := q.GetAll(c, &messages);
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	inboxItems := make([]inboxItem,0,len(messages));
	for i,msg := range messages {
		body := template.HTML(msg.Body)
		if msg.BodyHtml != nil {
			body = template.HTML(msg.BodyHtml)
		}
		iItem := inboxItem{
				Key: keys[i].IntID(), 
				From: msg.FromDisplay,
				Subject: msg.Subject,
				Body: body,
				ImageUrls: &msg.ImageUrls,
				ReceivedDate: msg.ReceivedDate,
		}
		inboxItems = append(inboxItems, iItem)
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
	if err := inboxTemplate.Execute(w, templateData{Token: token, Messages: inboxItems}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var inboxTemplate = template.Must(template.New("inbox.html").ParseFiles("tmpl/inbox.html"))
