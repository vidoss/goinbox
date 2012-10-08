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

func getBrowserId(c appengine.Context, w http.ResponseWriter, req *http.Request) (string, error) {
	bidCookie, err := req.Cookie("browserid");
	if err != nil {
		low, _, err := datastore.AllocateIDs(c, "endpoint", nil, 1)
		if err != nil {
			return "", err
		}
		browserId := strconv.FormatInt(low, 10)
		cookie := http.Cookie{ Name: "browserid", Value: browserId }
		http.SetCookie(w, &cookie);
		return browserId, nil
	}
	return bidCookie.Value, nil
}

func getMessages(w http.ResponseWriter, req *http.Request) {

	defer req.Body.Close()

	c := appengine.NewContext(req)

	browserId,err := getBrowserId(c, w, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

	// Cannel token
	token, err := channel.Create(c, browserId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Template
	if err := inboxTemplate.Execute(w, templateData{Token: token, Messages: inboxItems}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var inboxTemplate = template.Must(template.New("inbox.html").ParseFiles("tmpl/inbox.html"))
