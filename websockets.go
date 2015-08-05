package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"strings"
	"time"
)

//Thanks to Fatih Arslan for this. https://groups.google.com/d/msg/golang-nuts/KBx9pDlvFOc/0tR1gBRfFVMJ
//But it turns out that it is incomplete and do not work on Firefox based browsers that sends
// "Connection: keep-alive, upgrade" failing original tests.
func isWebsocket(req *http.Request) bool {
	conn_hdr := ""
	conn_hdrs := req.Header["Connection"]
	if len(conn_hdrs) > 0 {
		conn_hdr = conn_hdrs[0]
	}

	conn_hdr_parts := strings.Split(conn_hdr, ",")
	for _, part := range conn_hdr_parts {
		logger.WithField("part", part).Debug("Connection part")
		if strings.ToLower(strings.Trim(part, ", ")) == "upgrade" {
			return hasUpgrade(req)
		}
	}
	return false
}

func hasUpgrade(req *http.Request) bool {
	upgrade_websocket := false

	upgrade_hdrs := req.Header["Upgrade"]
	if len(upgrade_hdrs) > 0 {
		upgrade_websocket = (strings.ToLower(upgrade_hdrs[0]) == "websocket")
	}

	return upgrade_websocket
}

type websocketProxy struct{}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

func (wp websocketProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Upgrading websocket connection.")
	oconn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error(err)
		return
	}
	defer oconn.Close()

	r.URL.Host = config.Destination
	r.URL.Scheme = "ws"
	dialer := websocket.Dialer{
		HandshakeTimeout: time.Second,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		Subprotocols:     websocket.Subprotocols(r),
	}
	logger.Debug("Dialing", r.URL.String(), "...")
	iconn, _, err := dialer.Dial(r.URL.String(), nil)
	if err != nil {
		logger.Error(err)
		return
	}
	defer iconn.Close()
	logger.Debug("Connected!")

	go wp.Copy(oconn, iconn)
	wp.Copy(iconn, oconn)

}

func (wp websocketProxy) Copy(dst, src *websocket.Conn) {
	typ, message, err := src.ReadMessage()
	for err == nil {
		dst.WriteMessage(typ, message)
		typ, message, err = src.ReadMessage()
	}
	if err != io.EOF {
		logger.WithFields(logrus.Fields{
			"error":       err,
			"source":      src.RemoteAddr(),
			"destination": dst.RemoteAddr(),
		}).Error("Error while proxying WS data.")
	} else {
		logger.WithFields(logrus.Fields{
			"source":      src.RemoteAddr(),
			"destination": dst.RemoteAddr(),
		}).Debug("Source closed connection during WS proxying.")
	}
}
