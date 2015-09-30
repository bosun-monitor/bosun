package main

import (
	"bosun.org/_third_party/github.com/godbus/dbus"
	"bosun.org/_third_party/github.com/godbus/dbus/introspect"
	"encoding/json"
	"os"
)

func main() {
	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}
	node, err := introspect.Call(conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus"))
	if err != nil {
		panic(err)
	}
	data, _ := json.MarshalIndent(node, "", "    ")
	os.Stdout.Write(data)
}
