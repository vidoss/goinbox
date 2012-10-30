package inmail

import (
	"appengine"
	"appengine/blobstore"
	"appengine/channel"
	"appengine/datastore"
	"appengine/image"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"qprintable"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Message implements smtpd.Envelope by streaming the message to all
// connected websocket clients.
type Message struct {
	// HTML-escaped fields sent to the client
	From, To          string
	FromDisplay       string `datastore:",noindex"`
	Subject           string
	Body              string   `datastore:",noindex"`
	BodyHtml          []byte   `datastore:",noindex"`
	ImageUrls         []string `datastore:",noindex"`
	ReceivedDate      time.Time
	DeleteUnreadCount int64

	// internal state
	images []img_attachment
	bodies []string
	buf    bytes.Buffer // for accumulating email as it comes in
	msg    interface{}  // alternate message to send
}

type img_attachment struct {
	Type string
	Data []byte
}

func init() {
	http.HandleFunc("/_ah/mail/", incomingMail)
}

func incomingMail(w http.ResponseWriter, r *http.Request) {

	c := appengine.NewContext(r)
	defer r.Body.Close()

	//c.Infof("Inside incomingMail");

	var msg Message
	//c.Infof("before parse");
	err := msg.parse(c, r.Body)
	if err != nil {
		c.Errorf("Error parsing mail: %v", err)
		return
	}

	_, err = datastore.Put(c, datastore.NewIncompleteKey(c, "Message", nil), &msg)
	if err != nil {
		c.Errorf("Error saving mail: %v", err)
		return
	}

	if err != nil {
		c.Errorf("Error converting images to url: %v", err)
	}

	err = publishToChannels(c, &msg)
	if err != nil {
		c.Errorf("Error publishing to channels: %v", err)
		return
	}
}

func publishToChannels(c appengine.Context, msg *Message) error {
	q := datastore.NewQuery("Endpoint").KeysOnly()
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

func getFromDisplay(emailAddress string) string {
	reg, err := regexp.Compile("\\<.*?\\>")
	if err != nil {
		return "Jimmy Anon"
	}
	display := reg.Find([]byte(emailAddress))
	if display != nil {
		return strings.Replace(emailAddress, string(display), "", -1)
	}
	return strings.Split(emailAddress, "@")[0]
}

func (m *Message) parse(c appengine.Context, r io.Reader) error {
	//c.Infof("Inside parse")
	msg, err := mail.ReadMessage(r)
	if err != nil {
		c.Errorf("Error mail.ReadMessaage(): %v", err)
		return err
	}
	m.Subject = msg.Header.Get("Subject")
	m.From = msg.Header.Get("From")
	m.To = msg.Header.Get("To")
	m.FromDisplay = getFromDisplay(m.From)
	m.ReceivedDate = time.Now()

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		c.Errorf("Error mail.ParseMediaType(): %v", err)
		slurp, _ := ioutil.ReadAll(msg.Body)
		m.Body = string(slurp)
		return nil
	}
	//c.Infof("before parseMultipart()")
	if err := m.parseMultipart(c, msg.Body, params["boundary"]); err != nil {
		c.Errorf("Error mail.ParseMultiPart(): %v", err)
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
	// dump image attachments to blob store and get the urls.
	//c.Infof("before images2urls()")
	m.images2urls(c)

	return nil
}

func (m *Message) images2urls(c appengine.Context) {
	urlc := make(chan string)
	for _, im := range m.images {
		go func(img *img_attachment) {
			w, err := blobstore.Create(c, img.Type)
			if err != nil {
				urlc <- err.Error()
				return
			}
			_, err = w.Write(img.Data)
			if err != nil {
				urlc <- err.Error()
				return
			}
			err = w.Close()
			if err != nil {
				urlc <- err.Error()
				return
			}
			key, err := w.Key()
			if err != nil {
				urlc <- err.Error()
				return
			}
			url, err := image.ServingURL(c, key, &image.ServingURLOptions{Secure: false, Size: 0, Crop: false})
			if err != nil {
				urlc <- err.Error()
				return
			}
			urlc <- url.String()
		}(&im)
	}

	imagescnt := len(m.images)
	m.ImageUrls = make([]string, 0, imagescnt)
	for i := 0; i < imagescnt; i++ {
		u := <-urlc
		if strings.HasPrefix(u, "http") {
			m.ImageUrls = append(m.ImageUrls, u)
		} else {
			c.Errorf("Error converting image to url: %v", u)
		}
	}
	return
}

// parseMultipart populates Body (preferring text/plain) and images,
// and may call itself recursively, walking through multipart/mixed
// and multipart/alternative innards.
func (m *Message) parseMultipart(c appengine.Context, r io.Reader, boundary string) error {
	//c.Infof("inside parseMultipart()")
	mr := multipart.NewReader(r, boundary)
	for {
		//c.Infof("before nextPart()")
		part, err := mr.NextPart()
		if err == io.EOF {
			//c.Infof("NextPart(): Got EOF")
			break
		}
		if err != nil {
			c.Errorf("Error NextPart(): %v", err)
			return err
		}
		partType, partParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		//c.Infof("ParseMediaType(): %v",partType)
		if strings.HasPrefix(partType, "multipart/") {
			err = m.parseMultipart(c, part, partParams["boundary"])
			if err != nil {
				c.Errorf("in boundary %q, returning error for multipart child %q: %v", boundary, partParams["boundary"], err)
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
				c.Errorf("image base64 decode error: %v", err)
				continue
			}
			m.images = append(m.images, img_attachment{
				Type: partType,
				Data: imdata,
			})
			continue
		}
		if !strings.HasPrefix(partType, "text/") {
			continue
		}
		slurp, _ := ioutil.ReadAll(part)
		switch {
		case partType == "text/plain":
			m.Body = string(slurp)
			break

		case partType == "text/html":
			if part.Header.Get("Content-Transfer-Encoding") == "quoted-printable" {
				reader := qprintable.NewQuotedPrintableDecoder(bytes.NewBuffer(slurp))
				chunk, err := ioutil.ReadAll(reader)
				if err != nil {
					c.Errorf("Quated printable decode error: %v", err)
					continue
				}
				m.BodyHtml = chunk
			} else {
				m.BodyHtml = slurp
			}
			break

		default:
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
