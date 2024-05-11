// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpcplus

/*
	Some HTML presented at http://machine:port/debug/rpcplus
	Lists services, their methods, and some statistics, still rudimentary.
*/

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
)

const debugText = `<html>
	<body>
	<title>Services</title>
	<table border=1 cellpadding=5>
	<th align=center>Service</th><th align=center>MethodType</th><th align=center>Calls</th>
	{{range .}}
			<tr>
			<td align=left font=fixed>{{.Name}}</td>
			<td align=left font=fixed>{{.Type}}</td>
			<td align=center>{{.Service.NumCalls}}</td></tr>
	{{end}}
	</table>
	</body>
	</html>`

var debug = template.Must(template.New("RPCPlus debug").Parse(debugText))

// If set, print log statements for internal and I/O errors.
var debugLog = false

type debugService struct {
	Service *methodType
	Name    string
	Type    string
}

type serviceArray []debugService

func (s serviceArray) Len() int           { return len(s) }
func (s serviceArray) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s serviceArray) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type debugHTTP struct {
	*Server
}

// Runs at /debug/rpcplus
func (server debugHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Build a sorted version of the data.
	var services serviceArray
	server.serviceMap.Range(func(snamei, svci any) bool {
		// todo list all the method
		services = append(services, debugService{
			Service: svci.(*methodType),
			Name:    snamei.(string),
			Type:    svci.(*methodType).function.Type().String(),
		})

		return true
	})
	sort.Sort(services)
	err := debug.Execute(w, services)
	if err != nil {
		fmt.Fprintln(w, "rpcplus: error executing template:", err.Error())
	}
}
