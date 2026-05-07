package main

import (
	"encoding/xml"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/godbus/dbus/v5"
)

type entryKind int

const (
	entryBus entryKind = iota
	entryBusName
	entryObject
	entryInterface
	entryMethod
	entryProperty
	entrySignal
)

type entry struct {
	name        string
	detail      string
	path        string
	busName     string
	objectPath  string
	interface_  string
	busType     string
	kind        entryKind
	args        []introspectArg
	annotations []introspectAnnotation
	value       string
	memberName  string
}

type pane struct {
	title   string
	entries []entry
	cursor  int
	scroll  int
	err     error
}

type logEntry struct {
	time    time.Time
	message string
}

type callModal struct {
	method         entry
	inputs         []textinput.Model
	focus          int
	response       []string
	responseScroll int
	err            string
}

type model struct {
	help      help.Model
	width     int
	height    int
	conn      *dbus.Conn
	panes     []pane
	logs      []logEntry
	logScroll int
	modal     *callModal
}

type introspectNode struct {
	XMLName    xml.Name              `xml:"node"`
	Name       string                `xml:"name,attr"`
	Nodes      []introspectChildNode `xml:"node"`
	Interfaces []introspectInterface `xml:"interface"`
}

type introspectChildNode struct {
	Name string `xml:"name,attr"`
}

type introspectInterface struct {
	Name        string                 `xml:"name,attr"`
	Methods     []introspectMember     `xml:"method"`
	Signals     []introspectMember     `xml:"signal"`
	Properties  []introspectProperty   `xml:"property"`
	Annotations []introspectAnnotation `xml:"annotation"`
}

type introspectMember struct {
	Name        string                 `xml:"name,attr"`
	Args        []introspectArg        `xml:"arg"`
	Annotations []introspectAnnotation `xml:"annotation"`
}

type introspectArg struct {
	Name      string `xml:"name,attr"`
	Type      string `xml:"type,attr"`
	Direction string `xml:"direction,attr"`
}

type introspectProperty struct {
	Name        string                 `xml:"name,attr"`
	Type        string                 `xml:"type,attr"`
	Access      string                 `xml:"access,attr"`
	Annotations []introspectAnnotation `xml:"annotation"`
}

type introspectAnnotation struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}
