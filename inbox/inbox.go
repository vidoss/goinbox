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

func getChannelToken(c appengine.Context, w http.ResponseWriter, req *http.Request) (string, error) {
	tokenCookie, err := req.Cookie("token");
	if err != nil {
		low, _, err := datastore.AllocateIDs(c, "endpoint", nil, 1)
		if err != nil {
			return "", err
		}
		browserId := strconv.FormatInt(low, 10)
		token, err := channel.Create(c, browserId)
		if err != nil {
			return "", err
		}
		cookie := http.Cookie{ Name: "token", Value: token }
		http.SetCookie(w, &cookie);
		return token, nil
	}
	return tokenCookie.Value, nil
}

func getMessages(w http.ResponseWriter, req *http.Request) {

	defer req.Body.Close()

	c := appengine.NewContext(req)

	token,err := getChannelToken(c, w, req)
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
			//FIXME: Escape </textarea>
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

	// Template
	if err := inboxTemplate.Execute(w, templateData{Token: token, Messages: inboxItems}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var inboxTemplate = template.Must(template.New("inbox.html").ParseFiles("tmpl/inbox.html"))
