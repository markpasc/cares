package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	//"github.com/moovweb/gokogiri/xml"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Subscription struct {
	Id         uint64
	Url        string
	LeaseUntil time.Time
	Secret     sql.NullString
	Created    time.Time
}

func (s *Subscription) Notify(feed string) {
	// Don't notify if the subscription expired.
	if s.LeaseUntil.Before(time.Now().UTC()) {
		return
	}

	buf := bytes.NewBufferString(feed)
	req, err := http.NewRequest("POST", s.Url, buf)
	if err != nil {
		// ...?!?
		logr.Errln("Error creating new HTTP request to notify subscriber:", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/atom+xml")
	if s.Secret.Valid {
		sign := hmac.New(sha1.New, []byte(s.Secret.String))
		sign.Write([]byte(feed))
		hashData := make([]byte, 40, 0) // actually too long, it's 40 hex chars
		hash := sign.Sum(hashData)

		signature := fmt.Sprintf("sha1=%s", hex.EncodeToString(hash))
		req.Header.Set("X-Hub-Signature", signature)
	}

	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
}

func (s *Subscription) Save() error {
	if s.Id == 0 {
		return db.Insert(s)
	}
	_, err := db.Update(s)
	return err
}

func ActiveSubscriptions() ([]*Subscription, error) {
	rows, err := db.Select(Subscription{},
		"SELECT id, url, leaseuntil, secret, created FROM subscription WHERE leaseuntil > $1",
		time.Now().UTC())
	if err != nil {
		return nil, err
	}

	clouds := make([]*Subscription, len(rows))
	for i, row := range rows {
		clouds[i] = row.(*Subscription)
	}
	return clouds, nil
}

func NotifySubscribers(feed string) {
	logr.Debugln("Sending PubSubHubbub notifications")

	subs, err := ActiveSubscriptions()
	if err != nil {
		logr.Errln("Error finding pubsubhubbub subscribers:", err.Error())
		return
	}

	for _, sub := range subs {
		go sub.Notify(feed)
	}
}

type SubscribeRequest struct {
	Mode        string
	Topic       string
	CallbackUrl *url.URL
	LeaseUntil  time.Time
	Secret      string
	VerifyToken string
}

type UnverifiedResponse string

func (u UnverifiedResponse) Error() string {
	return string(u)
}

func (req *SubscribeRequest) Verify() error {
	challenge := "yup"

	query := req.CallbackUrl.Query()
	query.Set("hub.mode", req.Mode)
	query.Set("hub.topic", req.Topic)
	query.Set("hub.challenge", challenge)
	if req.Mode == "subscribe" {
		leaseDuration := req.LeaseUntil.Sub(time.Now().UTC())
		leaseSecondsStr := strconv.Itoa(int(leaseDuration / time.Second))
		query.Set("hub.lease_seconds", leaseSecondsStr)
	}
	if req.VerifyToken != "" {
		query.Set("hub.verify_token", req.VerifyToken)
	}

	verifyUrl := *req.CallbackUrl // verifyUrl is not a pointer
	verifyUrl.RawQuery = query.Encode()

	resp, err := http.Get(verifyUrl.String())
	if err != nil {
		return fmt.Errorf("Unexpected HTTP error verifying request")
	}

	// Ideally we would search progressively, such as by using
	// regexp.MatchReader(RuneReadifier(resp.Body))... but we'd have to
	// implement RuneReadifier.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if !bytes.Contains(body, []byte(challenge)) {
		return UnverifiedResponse("Response body did not contain the verification token")
	}

	subSecret := sql.NullString{req.Secret, req.Secret != ""}
	sub := &Subscription{0, req.CallbackUrl.String(), req.LeaseUntil, subSecret, time.Now().UTC()}
	err = db.Insert(sub)
	if err != nil {
		return err
	}

	return nil
}

func hub(w http.ResponseWriter, r *http.Request) {
	logr.Debugln("Yay, a pubsubhubbub request!")

	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "POST is required", http.StatusMethodNotAllowed)
		return
	}

	verifyModes := r.Form["hub.verify"]
	canVerifySync, canVerifyAsync := false, false
	for _, verifyMode := range verifyModes {
		if verifyMode == "sync" {
			canVerifySync = true
		} else if verifyMode == "async" {
			canVerifyAsync = true
		}
	}
	if !canVerifyAsync && !canVerifySync {
		logr.Debugln("Subscriber asked for verification modes", verifyModes, "so couldn't subscribe them")
		http.Error(w, fmt.Sprintf("None of your requested verification modes (%s) are supported", strings.Join(verifyModes, ",")), http.StatusBadRequest)
		return
	}

	topic := r.FormValue("hub.topic")
	// TODO: ssl?
	if topic != fmt.Sprintf("http://%s/atom", r.Host) {
		logr.Debugln("Subscriber asked for a subscription to", topic, "so couldn't subscribe them")
		http.Error(w, fmt.Sprintf("Your requested subscription topic %s is not tracked by this hub", topic), http.StatusBadRequest)
		return
	}

	callback := r.FormValue("hub.callback")
	callbackUrl, err := url.Parse(callback)
	if err != nil {
		logr.Debugln("Subscriber asked for a subscription with callback", callback, "which doesn't parse as an url:", err.Error())
		http.Error(w, fmt.Sprintf("Could not parse your callback URL %s", callback), http.StatusBadRequest)
		return
	}
	if callbackUrl.Scheme != "http" && callbackUrl.Scheme != "https" {
		logr.Debugln("Subscriber asked for a subscription with callback", callback, "which has unknown scheme", callbackUrl.Scheme)
		http.Error(w, fmt.Sprintf("Your callback URL's scheme %s is not supported", callbackUrl.Scheme), http.StatusBadRequest)
		return
	}
	if callbackUrl.Fragment != "" {
		logr.Debugln("Subscriber asked for a subscription with callback", callback, "which has a fragment", callbackUrl.Fragment)
		http.Error(w, fmt.Sprintf("Your callback URL has a fragment (%s) which is not supported", callbackUrl.Fragment), http.StatusBadRequest)
		return
	}

	leaseUntil := time.Now().UTC()
	leaseSecondsStr := r.FormValue("hub.lease_seconds")
	if leaseSecondsStr != "" {
		leaseSeconds, err := strconv.Atoi(leaseSecondsStr)
		if err != nil {
			logr.Debugln("Subscriber asked for a subscription with lease seconds", leaseSecondsStr, "so didn't subscribe")
			http.Error(w, fmt.Sprintf("Could not parse your requested lease seconds (%s)", leaseSecondsStr), http.StatusBadRequest)
			return
		}
		leaseUntil = leaseUntil.Add(time.Duration(leaseSeconds) * time.Second)
	} else {
		leaseUntil = leaseUntil.AddDate(0, 1, 0) // one month
	}

	req := &SubscribeRequest{
		r.FormValue("hub.mode"),
		topic,
		callbackUrl,
		leaseUntil,
		r.FormValue("hub.secret"),
		r.FormValue("hub.verify_token"),
	}

	if canVerifyAsync {
		go req.Verify()
		w.WriteHeader(http.StatusAccepted)
		return
	}

	err = req.Verify()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
