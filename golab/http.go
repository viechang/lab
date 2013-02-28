// Copyright 2013 Martin Schnabel. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/mb0/lab/golab/gosrc"
	"github.com/mb0/lab/golab/hub"
)

var (
	httpstart = flag.Bool("http", false, "start http server")
	httpaddr  = flag.String("addr", "localhost:8910", "http server addr")
)

func router(h *hub.Hub, m hub.Msg, id hub.Id) {
	// echo messages
	h.SendMsg(m, id)
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method nod allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexbytes)
}

func init() {
	flag.Parse()
	if !*httpstart {
		return
	}
	modules = append(modules, mainhttp)
}

func mainhttp() {
	h := hub.New(router)
	lab.src.SignalReports(func(r *gosrc.Report) {
		m, err := hub.Marshal("report", r)
		if err != nil {
			log.Println(err)
			return
		}
		h.SendMsg(m, hub.Group)
	})
	http.Handle("/ws", h)
	http.HandleFunc("/", index)
	log.Printf("starting http://%s/\n", *httpaddr)
	err := http.ListenAndServe(*httpaddr, nil)
	if err != nil {
		log.Fatalf("http %s\n", err)
	}
}

var indexbytes = []byte(`<!DOCTYPE html>
<html lang="en"><head>
	<meta charset="utf-8">
	<title>golab</title>
	<style>
	body { padding: 0; margin: 0; }
	.report.ok, .report.ok .status {
		background-color: green;
	}
	.report.fail, .report.fail .status {
		background-color: red;
	}
	.report header {
		background-color: white;
	}
	.report .status {
		display: inline-block;
		width: 40px;
	}
	.report .mode {
		display: inline-block;
		width: 50px;
	}
	.report pre {
		background-color: white;
		margin: 0 0 0 40px;
		padding-left: 5px;
	}
	</style>
</head><body><header>golab</header>
<script>(function() {
function newchild(pa, tag, inner) {
	var ele = document.createElement(tag);
	pa.appendChild(ele);
	if (inner) ele.innerHTML = inner;
	return ele;
}
function pad(str, l) {
	while (str.length < l) {
		str = "         " + str;
	}
	return str.slice(str.length-l);
}
function addreport(cont, r) {
	var c = newchild(cont, "div");
	header = '<span class="mode">'+r.Mode+'</span> '+ r.Path;
	if (r.Err != null) {
		c.setAttribute("class", "report fail");
		header = '<span class="status">FAIL</span> '+ header +": "+ r.Err;
	} else {
		c.setAttribute("class", "report ok");
		header = '<span class="status">ok</span> '+ header;
	}
	newchild(c, "header", header);
	var buf = [];
	if (r.Stdout) buf.push(r.Stdout);
	if (r.Stderr) buf.push(r.Stderr);
	var output = buf.join("\n")
	if (output) newchild(c, "pre", output);
	return c;
}
var cont = newchild(document.body, "div");
if (window["WebSocket"]) {
	var conn = new WebSocket("ws://"+ location.host+"/ws");
	conn.onclose = function(e) {
		newchild(cont, "div", "WebSocket closed.");
	};
	conn.onmessage = function(e) {
		var msg = JSON.parse(e.data);
		if (msg.Head == "report") {
			addreport(cont, msg.Data);
		} else {
			newchild(cont, "div", e.data);
		}
	};
	conn.onopen = function(e) {
		newchild(cont, "div", "WebSocket started.");
		conn.send('{"Head":"hi"}\n');
	};
} else {
	newchild(cont, "p", "WebSockets are not supported by your browser.");
}
})()</script></body></html>
`)