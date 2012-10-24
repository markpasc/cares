package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/moovweb/gokogiri"
	"github.com/moovweb/gokogiri/xml"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type RssCloudRequest struct {
	RequestMethodName string
	MethodName        string
	Host              string
	Port              uint16
	Path              string
	IsXmlRpc          bool
	FeedURL           string
}

func (r *RssCloudRequest) Unpack(doc *xml.XmlDocument) error {
	root := doc.Root()

	requestMethods, err := root.Search("methodName/text()")
	if err != nil {
		return err
	}
	if len(requestMethods) != 1 {
		return fmt.Errorf("Could not find cloud request's request method")
	}
	r.RequestMethodName = requestMethods[0].Content()

	// Would be nice to get the child nodes but xpath will give elements only, not text nodes.
	params, err := root.Search("params/param/value")
	if err != nil {
		return err
	}
	if len(params) != 5 {
		return fmt.Errorf("Could not unpack cloud request with %d params", len(params))
	}

	node := params[0].FirstChild()
	if node.NodeType() != xml.XML_TEXT_NODE {
		return fmt.Errorf("Could not unpack cloud request with first param not text")
	}
	r.MethodName = node.Content()

	node = params[1].FirstChild()
	if node.NodeType() != xml.XML_ELEMENT_NODE {
		return fmt.Errorf("Could not unpack cloud request with second param not an element")
	}
	if !(node.Name() == "i4" || node.Name() == "int") {
		return fmt.Errorf("Could not unpack cloud request with second param a %s element, not int or i4", node.Name())
	}
	node = node.FirstChild()
	if node.NodeType() != xml.XML_TEXT_NODE {
		return fmt.Errorf("Could not unpack cloud request with second param not containing text")
	}
	port, err := strconv.Atoi(node.Content())
	if err != nil {
		return err
	}
	r.Port = uint16(port)

	node = params[2].FirstChild()
	if node.NodeType() != xml.XML_TEXT_NODE {
		return fmt.Errorf("Could not unpack cloud request with third param not text")
	}
	r.Path = node.Content()

	node = params[3].FirstChild()
	if node.NodeType() != xml.XML_TEXT_NODE {
		return fmt.Errorf("Could not unpack cloud request with fourth param not text")
	}
	r.IsXmlRpc = strings.TrimSpace(node.Content()) == "xml-rpc"

	node = params[4].FirstChild()
	if node.NodeType() != xml.XML_ELEMENT_NODE {
		return fmt.Errorf("Could not unpack cloud request with fifth param not an element")
	}
	if node.Name() != "array" {
		return fmt.Errorf("Could not unpack cloud request with fifth param a %s element, not array", node.Name())
	}
	params, err = node.Search("data/value/text()")
	if err != nil {
		return err
	}
	if len(params) < 1 {
		return fmt.Errorf("Could not unpack cloud request with fifth param containing no data values")
	}
	r.FeedURL = params[0].Content()

	logr.Debugln("Unpacked cloud request!")
	return nil
}

type RssCloud struct {
	Id              uint64
	URL             string
	Method          string
	SubscribedUntil time.Time
	Created         time.Time
}

func NewRssCloud() *RssCloud {
	return &RssCloud{0, "", "", time.Unix(0, 0), time.Now().UTC()}
}

func (r *RssCloud) Notify(feedurl string) {
	logr.Debugln("Building RSS cloud notification for", r.URL)

	body := new(bytes.Buffer)
	body.WriteString(`<?xml version="1.0"?>
		<methodCall>
			<methodName>`)
	body.WriteString(r.Method)
	body.WriteString(`</methodName>
			<params>
				<param>
					<value>`)
	body.WriteString(feedurl)
	body.WriteString(`</value>
				</param>
			</params>
		</methodCall>`)

	resp, err := http.Post(r.URL, "text/xml", body)
	if err != nil {
		logr.Errln("Error posting RSS cloud notification to", r.URL, ":", err.Error())
		return
	}
	// TODO: parse resp and see if it was a fault.
	if resp == nil {
		logr.Errln("Didn't get a response back posting RSS cloud notification", r.URL, "!!!")
		return
	}

	logr.Debugln("Sent RSS cloud notification to", r.URL)
}

func (r *RssCloud) Save() error {
	if r.Id == 0 {
		return db.Insert(r)
	}
	_, err := db.Update(r)
	return err
}

func RssCloudByURL(url string) (*RssCloud, error) {
	rows, err := db.Select(RssCloud{},
		"SELECT id, method, subscribedUntil, created FROM rsscloud WHERE url = $1",
		url)
	if err != nil {
		return nil, err
	}
	rssCloud := rows[0].(*RssCloud)
	return rssCloud, nil
}

func ActiveRssClouds() ([]*RssCloud, error) {
	rows, err := db.Select(RssCloud{},
		"SELECT id, url, method, subscribedUntil, created FROM rsscloud WHERE subscribedUntil > $1",
		time.Now().UTC())
	if err != nil {
		return nil, err
	}

	clouds := make([]*RssCloud, len(rows))
	for i, row := range rows {
		clouds[i] = row.(*RssCloud)
	}
	return clouds, nil
}

func NotifyRssCloud(feedurl string) {
	logr.Debugln("Sending RSS cloud notifications")

	clouds, err := ActiveRssClouds()
	if err != nil {
		logr.Errln("Error finding RSS clouds to notify of feed update:", err.Error())
		return
	}

	for _, cloud := range clouds {
		go cloud.Notify(feedurl)
	}
}

func writeXmlRpcError(w http.ResponseWriter, err error) {
	logr.Errln("Error serving rss cloud request:", err.Error())
	output := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
		<methodResponse>
		<fault>
		  <value>
		     <struct>
		        <member>
		           <name>faultCode</name>
		           <value><int>4</int></value>
		           </member>
		        <member>
		           <name>faultString</name>
		           <value><string>%s</string></value>
		           </member>
		        </struct>
		     </value>
		  </fault>
		</methodResponse>`, err.Error())
	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(output)))
	w.Write([]byte(output))
}

func rssCloud(w http.ResponseWriter, r *http.Request) {
	logr.Debugln("Yay a cloud request!")

	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "POST is required", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes := make([]byte, r.ContentLength)
	_, err := r.Body.Read(bodyBytes)
	if err != nil {
		logr.Errln("Could not read request body:", err.Error())
		http.Error(w, "Could not read body: "+err.Error(), http.StatusInternalServerError)
		return
	}
	requestDoc, err := gokogiri.ParseXml(bodyBytes)
	if err != nil {
		writeXmlRpcError(w, err)
		return
	}

	request := new(RssCloudRequest)
	err = request.Unpack(requestDoc)
	if err != nil {
		writeXmlRpcError(w, err)
		return
	}

	hostname := r.Header.Get("X-Forwarded-For")
	if hostname == "" {
		hostname = r.RemoteAddr
	} else {
		hostnames := strings.SplitN(hostname, ",", 2)
		hostname = hostnames[0]
	}
	host, _, err := net.SplitHostPort(hostname)
	request.Host = host

	if request.RequestMethodName != "cloud.notify" {
		writeXmlRpcError(w, fmt.Errorf("Unknown method %s", request.RequestMethodName))
		return
	}

	if !request.IsXmlRpc {
		writeXmlRpcError(w, fmt.Errorf("Only XML-RPC is supported"))
		return
	}
	// TODO: use https as appropriate here? er, does river2 support cloud endpoints on HTTPS?
	if request.FeedURL != fmt.Sprintf("http://%s/rss", r.Host) {
		writeXmlRpcError(w, fmt.Errorf("RSS URL %s is not a feed managed here", request.FeedURL))
		return
	}

	logr.Debugln("Yay, asked to call back to http://", request.Host, ":", request.Port, request.Path,
		"with method", request.MethodName, "!")

	url, _ := url.Parse("/")
	if request.Port == 443 {
		url.Scheme = "https"
	} else {
		url.Scheme = "http"
	}
	if request.Port == 80 || request.Port == 443 {
		url.Host = request.Host
	} else {
		url.Host = net.JoinHostPort(request.Host, strconv.Itoa(int(request.Port)))
	}
	url.Path = request.Path
	urlString := url.String()

	rssCloud, err := RssCloudByURL(urlString)
	if err == sql.ErrNoRows {
		// That's cool.
	} else if err != nil {
		logr.Errln("Error loading rsscloud for URL", urlString, ":", err.Error())
		http.Error(w, "error looking for rsscloud for URL "+urlString, http.StatusInternalServerError)
		return
	}
	if rssCloud == nil {
		rssCloud = NewRssCloud()
		rssCloud.URL = urlString
	}
	rssCloud.Method = request.MethodName
	// Subscribe until 25 hours from now.
	rssCloud.SubscribedUntil = time.Now().Add(time.Duration(25) * time.Hour).UTC()
	err = rssCloud.Save()
	if err != nil {
		logr.Errln("Error saving rsscloud for URL", urlString, ":", err.Error())
		http.Error(w, "error saving rsscloud for URL "+urlString, http.StatusInternalServerError)
		return
	}

	output := `<?xml version="1.0" encoding="UTF-8"?>
		<methodResponse>
			<params>
				<param>
					<value><boolean>1</boolean></value>
				</param>
			</params>
		</methodResponse>`

	w.Header().Set("Content-Type", "text/xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(output)))
	w.Write([]byte(output))
}
