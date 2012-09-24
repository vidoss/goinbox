package inchan

import (
		  "net/http"
        "appengine"
        "appengine/datastore"
		  "strconv"
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
        if _, err := datastore.Put(c, datastore.NewKey(c, "endpoint", "", from, nil), &Endpoint{}); err != nil {
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
        if err := datastore.Delete(c, datastore.NewKey(c, "endpoint", "", from, nil)); err != nil {
				c.Errorf("Error datastore.Delete: %v", err)
                return
        }
        return
}

