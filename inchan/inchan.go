package inchan

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"inbox"
	"inmail"
	"net/http"
	"strconv"
	"encoding/json"
)

func init() {
	http.HandleFunc("/_ah/channel/connected/", channelConnected)
	http.HandleFunc("/_ah/channel/disconnected/", channelDisconnected)
}

type Endpoint struct {}

func channelConnected(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	from, err := strconv.ParseInt(r.FormValue("from"), 10, 0)
	if err != nil {
		c.Errorf("Error ParseInt: %v", err)
		return
	}
	if _, err := datastore.Put(c, datastore.NewKey(c, "Endpoint", "", from, nil), &Endpoint{}); err != nil {
		c.Errorf("Error datastore.Put: %v", err)
		return
	}
	return
}

func channelDisconnected(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	from, err := strconv.ParseInt(r.FormValue("from"), 10, 0)
	if err != nil {
		c.Errorf("Error ParseInt: %v", err)
		return
	}
	if err := datastore.Delete(c, datastore.NewKey(c, "Endpoint", "", from, nil)); err != nil {
		c.Errorf("Error datastore.Delete: %v", err)
		return
	}

	token, err := getToken(c, from)
	if err != nil {
		c.Errorf("Error getToken(): %v", err)
		return
	}
	//c.Infof("Token: %s", token)

	act, err := memcache.Get(c, token)
	if err != nil && err != memcache.ErrCacheMiss {
		c.Errorf("Error memcache.Get: %v", err)
		return
	}

	var actions []inbox.ActionItem
	if err == nil {
		//c.Infof("mcache: %s", act.Value)
		err = json.Unmarshal(act.Value, &actions)
		if err != nil {
			c.Errorf("Error datastore.Get: %v", err)
			return
		}
		for _, a := range actions {
			var mailItem = new (inmail.Message)
			mkey := datastore.NewKey(c, "Message", "", a.MessageKey, nil)
			if err = datastore.Get(c, mkey, mailItem); err != nil {
				c.Errorf("Error datastore.Get: %v", err)
				continue
			}
			//c.Infof("Cmd: %s",a.Cmd)
			switch a.Cmd {
				case "PUT":
					mailItem.DeleteUnreadCount++
				case "DELETE":
					mailItem.DeleteUnreadCount--
			}
			//c.Infof("DeleteUnreadCount: %d",mailItem.DeleteUnreadCount)
			if _, err = datastore.Put(c, mkey, mailItem); err != nil {
				c.Errorf("Error datastore.Get: %v", err)
			}
		}
	}

	return
}

func getToken(c appengine.Context, clientId int64) (string, error) {
	usr := new(inbox.EndpointUser)
	if err := datastore.Get(c, datastore.NewKey(c, "EndpointUser", "", clientId, nil), usr); err != nil {
		return "", err
	}
	return usr.Token, nil
}
