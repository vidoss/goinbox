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

var (
   templates = template.Must(template.ParseFiles(
      "tmpl/fullpage.html",
      "tmpl/messages.html",
      "tmpl/emails.html",
      "tmpl/error.html",
   ))
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
	http.HandleFunc("/", errorHandler(getInbox))
	http.HandleFunc("/inbox", errorHandler(getInbox))
	//http.HandleFunc("/trash", errorHandler(getTrash))
	//http.HandleFunc("/email/id", errorHandler(getTrash))
	//http.HandleFunc("/folder", errorHandler(getTrash))
	//http.HandleFunc("/folder/id", errorHandler(getTrash))
	//http.HandleFunc("/rules", errorHandler(getTrash))
	//http.HandleFunc("/rules/id", errorHandler(getTrash))
}

func errorHandler(fn func(http.ResponseWriter,*http.Request) error) http.HandlerFunc {
   return func(w http.ResponseWriter, r *http.Request) {

		defer r.Body.Close()

		err := fn(w, r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			templates.ExecuteTemplate(w, "error.html", err)
		}

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

func getInbox(w http.ResponseWriter, req *http.Request) error {
	
	c := appengine.NewContext(req)

	token,err := getChannelToken(c, w, req)
	if err != nil {
		return err
	}

	q := datastore.NewQuery("Message").Order("-ReceivedDate").Limit(50)

	messages := make([]inmail.Message, 0, 50)
	keys, err := q.GetAll(c, &messages);
	if err != nil {
		return err
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

	// Template
	err = templates.ExecuteTemplate(w, "fullpage.html", templateData{Token: token, Messages: inboxItems})
	if err != nil {
		return err
	}
	return nil;

}

