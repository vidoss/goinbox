package inmail

import (
	"appengine"
	"time"
	"appengine/datastore"
	"appengine/channel"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"strings"
	"strconv"
)

func init() {
	http.HandleFunc("/_ah/mail/", incomingMail)
}


func incomingMail(w http.ResponseWriter, r *http.Request) {

	c := appengine.NewContext(r)
	defer r.Body.Close()

	var msg Message
	err := msg.parse(r.Body)
	if err != nil {
		c.Errorf("Error parsing mail: %v", err)
		return
	}

	_, err = datastore.Put(c, datastore.NewIncompleteKey(c, "Message", nil), &msg)
   if err != nil {
		c.Errorf("Error saving mail: %v", err)
		return
    }

	err = publishToChannels(c, &msg)
	if err != nil {
		c.Errorf("Error publishing to channels: %v", err)
		return
	}
}

func publishToChannels(c appengine.Context, msg *Message) error {
        q := datastore.NewQuery("endpoint").KeysOnly()
        endpoints, err := q.GetAll(c, nil)
        if err != nil {
				c.Errorf("Error getting enpoints: %v", err)
            return err
        }
        message, err := json.Marshal(msg)
        if err != nil {
				c.Errorf("Error marshaling msg -> json: %v", err)
            return err
        }
        errc := make(chan error)
        for _, e := range endpoints {
                go func(e *datastore.Key) {
                        err := channel.Send(c, strconv.FormatInt(e.IntID(), 10), string(message))
                        if err != nil {
                                // Send failed. Delete the endpoint.
                                if err := datastore.Delete(c, e); err != nil {
                                        c.Errorf("deleting endpoint: %v", err)
                                }
                        }
                        errc <- err
                }(e)
        }
        for i := 0; i < len(endpoints); i++ {
                if err := <-errc; err != nil {
                        c.Errorf("sending message: %v", err)
                }
        }
        return nil
}

// Message implements smtpd.Envelope by streaming the message to all
// connected websocket clients.
type Message struct {
	// HTML-escaped fields sent to the client
	From, To string
	Subject  string
	Body     string 
	Images []image
	ReceivedDate time.Time

	// internal state
	bodies []string
	buf    bytes.Buffer // for accumulating email as it comes in
	msg    interface{}  // alternate message to send
}

type image struct {
	Type string
	Data []byte
}

func (m *Message) parse(r io.Reader) error {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return err
	}
	m.Subject = msg.Header.Get("Subject")
	m.From = msg.Header.Get("From")
	m.To = msg.Header.Get("To")
	m.ReceivedDate = time.Now()

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		slurp, _ := ioutil.ReadAll(msg.Body)
		m.Body = string(slurp)
		return nil
	}
	if err := m.parseMultipart(msg.Body, params["boundary"]); err != nil {
		return err
	}
	// If we didn't find a text/plain body, pick the first body we did find.
	if m.Body == "" {
		for _, body := range m.bodies {
			if body != "" {
				m.Body = body
				break
			}
		}
	}
	return nil
}

// parseMultipart populates Body (preferring text/plain) and images,
// and may call itself recursively, walking through multipart/mixed
// and multipart/alternative innards.
func (m *Message) parseMultipart(r io.Reader, boundary string) error {
	mr := multipart.NewReader(r, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		partType, partParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if strings.HasPrefix(partType, "multipart/") {
			err = m.parseMultipart(part, partParams["boundary"])
			if err != nil {
				log.Printf("in boundary %q, returning error for multipart child %q: %v", boundary, partParams["boundary"], err)
				return err
			}
			continue
		}
		if strings.HasPrefix(partType, "image/") {
			switch partType {
			case "image/gif", "image/png", "image/jpeg":
			default:
				// Probably http://golang.org/issue/3562
				continue
			}
			contentDis := part.Header.Get("Content-Disposition")
			if !(strings.HasPrefix(contentDis, "attachment") || strings.HasPrefix(contentDis, "inline")) {
				continue
			}
			if part.Header.Get("Content-Transfer-Encoding") != "base64" {
				continue
			}
			slurp, _ := ioutil.ReadAll(part)
			imdata, err := ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(removeNewlines(slurp))))
			if err != nil {
				log.Printf("image base64 decode error: %v", err)
				continue
			}
			m.Images = append(m.Images, image{
				Type: partType,
				Data: imdata,
			})
			continue
		}
		if !strings.HasPrefix(partType, "text/") {
			continue
		}
		slurp, _ := ioutil.ReadAll(part)
		if partType == "text/plain" {
			m.Body = string(slurp)
		} else {
			m.bodies = append(m.bodies, string(slurp))
		}
	}
	return nil
}

func removeNewlines(p []byte) []byte {
	return bytes.Map(func(r rune) rune {
		switch r {
		case '\n', '\r':
			return -1
		}
		return r
	}, p)
}
