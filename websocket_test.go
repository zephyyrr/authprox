package main

import (
	"net/http"
	"testing"
)

func TestIsWebsocket_Chrome(t *testing.T) {
	if isWebsocket(&http.Request{
		Header: http.Header{
			"Accept-Encoding":          []string{"gzip, deflate, sdch"},
			"Accept-Language":          []string{"sv,en-US;q=0.8,en;q=0.6"},
			"Cache-Control":            []string{"no-cache"},
			"Connection":               []string{"Upgrade"},
			"DNT":                      []string{"1"},
			"Host":                     []string{"localhost:1337"},
			"Origin":                   []string{"http://localhost:1337"},
			"Pragma":                   []string{"no-cache"},
			"Sec-WebSocket-Extensions": []string{"permessage-deflate; client_max_window_bits"},
			"Sec-WebSocket-Key":        []string{"oGfEAZmfO5EsHn1whKQRTw=="},
			"Sec-WebSocket-Version":    []string{"13"},
			"Upgrade":                  []string{"websocket"},
			"User-Agent":               []string{"Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/44.0.2403.125 Safari/537.36"},
		},
	}) != true {
		t.Fail()
	}
}

func TestIsWebsocket_Firefox(t *testing.T) {
	if isWebsocket(&http.Request{
		Header: http.Header{
			"Host":                  []string{"localhost:1337"},
			"Connection":            []string{"Upgrade"},
			"Pragma":                []string{"no-cache"},
			"Cache-Control":         []string{"no-cache"},
			"Upgrade":               []string{"websocket"},
			"Origin":                []string{"http://localhost:1337"},
			"Sec-WebSocket-Version": []string{"13"},
			"DNT":                      []string{"1"},
			"User-Agent":               []string{"Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/44.0.2403.125 Safari/537.36"},
			"Accept-Encoding":          []string{"gzip, deflate, sdch"},
			"Accept-Language":          []string{"sv,en-US;q=0.8,en;q=0.6"},
			"Cookie":                   []string{"auth=MTQzODc3MDUyNnx5Zmpma1k3eS1UbnF5UjB6dm5kU2stTEhtYTZLeEVtZVlJUGxZalZIWVdIXzlvdDBZZzdYaElsLVZZUkVUemhXNi1PRGVPWUJDSmd0bVBYRk9tN3ZQVDM0U0dEUHg4UzdmUmw2TXd0TDJJRFZLaXJubkFUMmdCT1J0TzFpTDhSdUVaRT18LsNxLm20IJTaUH759MtWX2B6Y6TZXC_-VegMMwBzoVc="},
			"Sec-WebSocket-Key":        []string{"oGfEAZmfO5EsHn1whKQRTw=="},
			"Sec-WebSocket-Extensions": []string{"permessage-deflate; client_max_window_bits"},
		},
	}) != true {
		t.Fail()
	}
}
