package inbox

import (
	"appengine"
	"appengine/channel"
	"appengine/datastore"
	"appengine/memcache"
	"encoding/json"
	"html/template"
	"inmail"
	"net/http"
	"strconv"
	"strings"
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
	Messages []MailItem
}

type ActionItem struct {
	MessageKey int64
	Cmd        string
}

type MailItem struct {
	Key               int64
	From              string
	Subject           string
	Body              template.HTML
	ImageUrls         *[]string
	ReceivedDate      time.Time
	DeleteUnreadCount int64
}

type EndpointUser struct {
	Token string
}

func init() {
	http.HandleFunc("/", errorHandler(getInbox))
	http.HandleFunc("/inbox", errorHandler(getInbox))
	//http.HandleFunc("/trash", errorHandler(getTrash))
	http.HandleFunc("/email/", errorHandler(handleEmail))
	//http.HandleFunc("/folder", errorHandler(getTrash))
	//http.HandleFunc("/folder/id", errorHandler(getTrash))
	//http.HandleFunc("/rules", errorHandler(getTrash))
	//http.HandleFunc("/rules/id", errorHandler(getTrash))
}

func errorHandler(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		defer r.Body.Close()

		err := fn(w, r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			templates.ExecuteTemplate(w, "error.html", err)
		}

	}
}

func handleEmail(w http.ResponseWriter, req *http.Request) error {

	c := appengine.NewContext(req)

	cmd := req.FormValue("cmd")
	switch cmd {

	case "UNREAD", "DELETE":

		emailId,err := strconv.ParseInt(req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:],10,64)
		tknCookie, err := req.Cookie("token")
		if err != nil {
			return err
		}
		userid := tknCookie.Value

		act, err := memcache.Get(c, userid)
		if err != nil && err != memcache.ErrCacheMiss {
			return err
		}
		var actions []ActionItem
		if err == nil {
			//c.Infof("mcache: %s", act.Value)
			err = json.Unmarshal(act.Value, &actions)
			if err != nil {
				return err
			}
			var dedupActions []ActionItem
			for _, a := range actions {
				if a.Cmd != cmd || a.MessageKey != emailId {
					dedupActions = append(dedupActions, a)
				}
			}
			actions = dedupActions
		}

		actions = append(actions, ActionItem{Cmd: cmd, MessageKey: emailId})
		actJson, err := json.Marshal(actions)
		if err != nil {
			return err
		}
		//c.Infof("set: %s", actJson)
		newAction := &memcache.Item{
			Key:   userid,
			Value: actJson,
			Expiration: time.Hour*2,
		}
		if err := memcache.Set(c, newAction); err != nil {
			return err
		}
	}

	return nil
}

func getChannelToken(c appengine.Context, w http.ResponseWriter, req *http.Request) (string, error) {
	tokenCookie, err := req.Cookie("token")
	if err != nil {
		low, _, err := datastore.AllocateIDs(c, "Endpoint", nil, 1)
		if err != nil {
			return "", err
		}
		browserId := strconv.FormatInt(low, 10)
		token, err := channel.Create(c, browserId)
		if err != nil {
			return "", err
		}
		cookie := http.Cookie{Name: "token", Value: token, MaxAge: 7200} /* two hours */
		http.SetCookie(w, &cookie)

		epkey := datastore.NewKey(c, "EndpointUser", "", low, nil)
		if _, err := datastore.Put(c, epkey, &EndpointUser{Token: token}); err != nil {
			return "", err
		}

		return token, nil
	}
	return tokenCookie.Value, nil
}

func getInbox(w http.ResponseWriter, req *http.Request) error {

	c := appengine.NewContext(req)

	token, err := getChannelToken(c, w, req)
	if err != nil {
		return err
	}

	q := datastore.NewQuery("Message").Order("-DeleteUnreadCount").Order("-ReceivedDate").Limit(50)

	messages := make([]inmail.Message, 0, 50)
	keys, err := q.GetAll(c, &messages)
	if err != nil {
		return err
	}

	inboxItems := make([]MailItem, 0, len(messages))
	for i, msg := range messages {
		body := template.HTML(msg.Body)
		if msg.BodyHtml != nil {
			body = template.HTML(msg.BodyHtml)
		}
		iItem := MailItem{
			Key:          keys[i].IntID(),
			From:         msg.FromDisplay,
			Subject:      msg.Subject,
			Body:         body,
			ImageUrls:    &msg.ImageUrls,
			ReceivedDate: msg.ReceivedDate,
		}
		inboxItems = append(inboxItems, iItem)
	}

	// Template
	err = templates.ExecuteTemplate(w, "fullpage.html", templateData{Token: token, Messages: inboxItems})
	if err != nil {
		return err
	}
	return nil

}
